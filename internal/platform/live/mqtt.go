package live

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/presence"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Topic is where a project's hints are published: kyber/<workspace>/projects/<KEY>/events.
func Topic(workspace, project string) string {
	return "kyber/" + workspace + "/projects/" + project + "/events"
}

// UserTopic is where one user's own events go (S88: chat, calls, whiteboards):
// kyber/<workspace>/users/<user>/events, payload {"type","data"}.
func UserTopic(workspace, user string) string {
	return "kyber/" + workspace + "/users/" + user + "/events"
}

// CanBrowse reports whether user may see project of the workspace in ctx (the composition root
// asks the Project context).
type CanBrowse func(ctx context.Context, user, project string) bool

// Login authenticates a client connecting over plain MQTT: the username names the workspace (its
// id) and the password is a personal access token; it returns the user.
type Login func(ctx context.Context, workspace, token string) (string, error)

// Active reports whether user may still use Kyber (not deactivated or deleted, e.g. over SCIM); the
// composition root asks the Identity context. ctx carries the client's workspace.
type Active func(ctx context.Context, user string) bool

// identity is who a connected client acts for.
type identity struct{ user, workspace string }

// Broker is Kyber's embedded MQTT broker (ADR-0013). Clients only subscribe: to their
// workspace's project topics, and every delivery is re-authorized, so a revoked membership stops
// hints at once, and a deactivated user is disconnected on their next delivery. The server publishes through the inline client.
type Broker struct {
	srv      *mqtt.Server
	can      CanBrowse
	login    Login
	alive    Active
	mu       sync.Mutex
	who      map[*mqtt.Client]identity
	onChange func() // S105: someone connected or left (presence.Roster.Changed)
	up       websocket.Upgrader
}

const wsListener = "ws"

func NewBroker(can CanBrowse, login Login, active Active, log *slog.Logger) (*Broker, error) {
	srv := mqtt.New(&mqtt.Options{InlineClient: true, Logger: log})
	b := &Broker{srv: srv, can: can, login: login, alive: active, who: map[*mqtt.Client]identity{},
		up: websocket.Upgrader{Subprotocols: []string{"mqtt"}}} // default CheckOrigin: same origin only
	// Both fail only for duplicate ids, impossible on a fresh server.
	_ = srv.AddHook(&hook{b: b}, nil)
	_ = srv.AddListener(listeners.NewMockListener(wsListener, ""))
	return b, srv.Serve()
}

// ListenTCP accepts plain MQTT clients (e.g. :1883) authenticated by Login.
func (b *Broker) ListenTCP(addr string) error {
	l := listeners.NewTCP(listeners.Config{ID: "tcp", Address: addr})
	if err := b.srv.AddListener(l); err != nil {
		return err
	}
	go l.Serve(b.srv.EstablishConnection)
	return nil
}

func (b *Broker) Close() error { return b.srv.Close() }

// Publish delivers a hint to this replica's subscribers.
func (b *Broker) Publish(c Change) {
	payload, _ := json.Marshal(c)
	_ = b.srv.Publish(Topic(c.Workspace, c.Project), payload, false, 0)
}

// PublishTo delivers an event to the given users' own topics on this replica (S88).
func (b *Broker) PublishTo(workspace string, users []string, typ string, data []byte) {
	payload, _ := json.Marshal(struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}{typ, data})
	for _, u := range users {
		_ = b.srv.Publish(UserTopic(workspace, u), payload, false, 0)
	}
}

// OnChange is called whenever a client connects or disconnects. Set it before serving.
func (b *Broker) OnChange(f func()) { b.onChange = f }

func (b *Broker) changed() {
	if b.onChange != nil {
		b.onChange()
	}
}

// Seats lists the users connected to this broker (S105 presence across replicas).
func (b *Broker) Seats() []presence.Seat {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]presence.Seat, 0, len(b.who))
	for _, id := range b.who {
		out = append(out, presence.Seat{Workspace: id.workspace, User: id.user})
	}
	return out
}

// Online lists which of users have a client connected in workspace (in the given order).
func (b *Broker) Online(workspace string, users []string) []string {
	b.mu.Lock()
	on := map[string]bool{}
	for _, id := range b.who {
		if id.workspace == workspace {
			on[id.user] = true
		}
	}
	b.mu.Unlock()
	out := []string{}
	for _, u := range users {
		if on[u] && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
}

// Broadcast publishes to this process only (a single replica, or dev mode).
func (b *Broker) Broadcast(_ context.Context, c Change) error {
	b.Publish(c)
	return nil
}

// Handler upgrades an authenticated request (session cookie or token, behind the workspace
// middleware) to MQTT over WebSocket.
func (b *Broker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := identity{user: auth.Actor(r.Context()), workspace: tenant.Workspace(r.Context())}
		ws, err := b.up.Upgrade(hijacker{w, http.NewResponseController(w)}, r, nil)
		if err != nil {
			return // the upgrader has answered
		}
		conn := ws.UnderlyingConn()
		_ = conn.SetDeadline(time.Time{}) // drop the server's request timeouts; MQTT keepalive governs now
		_ = b.srv.EstablishConnection(wsListener, &wsConn{Conn: conn, c: ws, id: id})
	})
}

// hijacker reaches the connection through middleware writers that only offer Unwrap.
type hijacker struct {
	http.ResponseWriter
	rc *http.ResponseController
}

func (h hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) { return h.rc.Hijack() }

// wsConn carries MQTT packets in binary WebSocket messages and remembers who connected.
type wsConn struct {
	net.Conn
	c  *websocket.Conn
	r  io.Reader
	id identity
}

var errDeactivated = errors.New("user deactivated")

var errTextFrame = errors.New("mqtt over websocket needs binary messages")

func (ws *wsConn) Read(p []byte) (int, error) {
	for {
		if ws.r == nil {
			op, r, err := ws.c.NextReader()
			if err != nil {
				return 0, err
			}
			if op != websocket.BinaryMessage {
				return 0, errTextFrame
			}
			ws.r = r
		}
		n, err := ws.r.Read(p)
		if errors.Is(err, io.EOF) { // end of this message; the next Read starts the next one
			ws.r, err = nil, nil
		}
		if n == 0 && err == nil {
			continue
		}
		return n, err
	}
}

func (ws *wsConn) Write(p []byte) (int, error) {
	if err := ws.c.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (ws *wsConn) Close() error { return ws.c.Close() }

type hook struct {
	mqtt.HookBase
	b *Broker
}

func (h *hook) ID() string { return "kyber" }

func (h *hook) Provides(x byte) bool {
	return bytes.Contains([]byte{mqtt.OnConnectAuthenticate, mqtt.OnACLCheck, mqtt.OnDisconnect}, []byte{x})
}

// OnConnectAuthenticate identifies the client: WebSocket clients by the request that opened the
// socket, plain MQTT clients by Login. The client id is namespaced by user, so one user can never
// take over another's session.
func (h *hook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	var id identity
	if c, ok := cl.Net.Conn.(*wsConn); ok {
		id = c.id
	} else {
		ws := string(pk.Connect.Username)
		user, err := h.b.login(tenant.With(context.Background(), ws), ws, string(pk.Connect.Password))
		if err != nil {
			return false
		}
		id = identity{user: user, workspace: ws}
	}
	cl.ID = id.user + "/" + cl.ID
	h.b.mu.Lock()
	h.b.who[cl] = id
	h.b.mu.Unlock()
	h.b.changed()
	return true
}

func (h *hook) OnDisconnect(cl *mqtt.Client, _ error, _ bool) {
	h.b.mu.Lock()
	delete(h.b.who, cl)
	h.b.mu.Unlock()
	h.b.changed()
}

// OnACLCheck: clients never publish; they may subscribe to project topics ("+" may stand for
// their workspace or any project), and each delivery (a concrete topic) must be in their
// workspace and needs browse permission on that project.
func (h *hook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	if write {
		return false
	}
	h.b.mu.Lock()
	id := h.b.who[cl] // unknown clients get the zero identity, which matches no workspace
	h.b.mu.Unlock()
	if id.user != "" && !h.b.alive(tenant.With(context.Background(), id.workspace), id.user) {
		cl.Stop(errDeactivated) // a deactivated user's sessions and tokens are gone; end the stream too
		return false
	}
	parts := strings.Split(topic, "/")
	if len(parts) == 5 && parts[0] == "kyber" && parts[2] == "users" && parts[4] == "events" {
		// S88: own events only; "+" may stand for the workspace and the user, every delivery is concrete.
		return (parts[1] == id.workspace || parts[1] == "+") && (parts[3] == "+" || (parts[3] == id.user && id.user != ""))
	}
	if len(parts) != 5 || parts[0] != "kyber" || parts[2] != "projects" || parts[4] != "events" ||
		(parts[1] != id.workspace && parts[1] != "+") || parts[3] == "#" || parts[3] == "" {
		return false
	}
	if parts[1] == "+" || parts[3] == "+" {
		return true // a subscription; every delivery is a concrete topic checked below
	}
	return h.b.can(tenant.With(context.Background(), id.workspace), id.user, parts[3])
}
