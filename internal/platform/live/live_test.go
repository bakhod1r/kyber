package live_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/live"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

const (
	ws    = "00000000-0000-4000-8000-0000000000a1"
	other = "00000000-0000-4000-8000-0000000000a2"
)

func TestFromMessage(t *testing.T) {
	for _, tc := range []struct {
		payload string
		want    string
		ok      bool
	}{
		{`{"key":"KYB-12","title":"secret"}`, "KYB", true},
		{`{"key":"MY-APP-3"}`, "MY-APP", true},
		{`{"project":"OPS","id":"s-1"}`, "OPS", true},
		{`{"id":"x"}`, "", false},
		{`{"key":"nodash"}`, "", false},
		{`not json`, "", false},
	} {
		c, ok := live.FromMessage(outbox.Message{Name: "e", Workspace: "w-1", Payload: []byte(tc.payload)})
		if ok != tc.ok || c.Project != tc.want || (ok && (c.Workspace != "w-1" || c.Event != "e")) {
			t.Errorf("%s = %+v %v", tc.payload, c, ok)
		}
	}
}

// ann may browse KYB; ben may browse nothing. Tokens: "ann-token" (ann), "ben-token" (ben).
func broker(t *testing.T) *live.Broker {
	t.Helper()
	return brokerWith(t, func(context.Context, string) bool { return true })
}

// brokerWith is broker with the given check of whether a user is still active.
func brokerWith(t *testing.T, active live.Active) *live.Broker {
	t.Helper()
	revoked := map[string]bool{}
	can := func(ctx context.Context, user, project string) bool {
		w, _ := tenant.From(ctx)
		return w == ws && user == "ann" && project == "KYB" && !revoked[user]
	}
	login := func(_ context.Context, workspace, token string) (string, error) {
		if workspace != ws {
			return "", errors.New("not a member")
		}
		user, ok := map[string]string{"ann-token": "ann", "ben-token": "ben"}[token]
		if !ok {
			return "", errors.New("bad token")
		}
		return user, nil
	}
	b, err := live.NewBroker(can, login, active, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

type received chan paho.Message

// subscribe connects a client and subscribes to filter; it fails the test if the broker refuses.
func subscribe(t *testing.T, opts *paho.ClientOptions, filter string) (received, paho.Client) {
	t.Helper()
	got := make(received, 10)
	c := paho.NewClient(opts.SetConnectTimeout(3 * time.Second).SetAutoReconnect(false))
	if tok := c.Connect(); !tok.WaitTimeout(3*time.Second) || tok.Error() != nil {
		t.Fatalf("connect: %v", tok.Error())
	}
	t.Cleanup(func() { c.Disconnect(10) })
	tok := c.Subscribe(filter, 0, func(_ paho.Client, m paho.Message) { got <- m })
	if !tok.WaitTimeout(3*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe: %v", tok.Error())
	}
	return got, c
}

func (r received) next(t *testing.T) paho.Message {
	t.Helper()
	select {
	case m := <-r:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("no message")
		return nil
	}
}

func (r received) none(t *testing.T) {
	t.Helper()
	select {
	case m := <-r:
		t.Fatalf("unexpected %s %s", m.Topic(), m.Payload())
	case <-time.After(300 * time.Millisecond):
	}
}

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// Browsers speak MQTT over WebSocket as the user of the authenticated request; only hints of
// browsable projects are delivered.
func TestWebSocketClientsGetBrowsableHints(t *testing.T) {
	b := broker(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-User")
		b.Handler().ServeHTTP(w, r.WithContext(auth.WithActor(tenant.With(r.Context(), ws), user)))
	}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	filter := live.Topic("+", "+") // browsers need not know the workspace id
	ann, _ := subscribe(t, paho.NewClientOptions().AddBroker(url).SetClientID("web").SetHTTPHeaders(http.Header{"X-User": {"ann"}}), filter)
	ben, _ := subscribe(t, paho.NewClientOptions().AddBroker(url).SetClientID("web").SetHTTPHeaders(http.Header{"X-User": {"ben"}}), filter)

	b.Publish(live.Change{Workspace: ws, Project: "KYB", Event: "issue.created"})
	b.Publish(live.Change{Workspace: ws, Project: "OPS", Event: "issue.created"})
	b.Publish(live.Change{Workspace: other, Project: "KYB", Event: "issue.created"})
	m := ann.next(t)
	if m.Topic() != live.Topic(ws, "KYB") || string(m.Payload()) != `{"project":"KYB","event":"issue.created"}` {
		t.Fatalf("hint = %s %s", m.Topic(), m.Payload())
	}
	ann.none(t)
	ben.none(t)
	if err := b.Broadcast(context.Background(), live.Change{Workspace: ws, Project: "KYB", Event: "issue.changed"}); err != nil {
		t.Fatal(err)
	}
	if m := ann.next(t); !strings.Contains(string(m.Payload()), "issue.changed") {
		t.Fatalf("broadcast = %s", m.Payload())
	}
}

// Cross-site pages cannot open the socket with a visitor's cookie.
func TestWebSocketRejectsOtherOrigins(t *testing.T) {
	b := broker(t)
	srv := httptest.NewServer(b.Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	for k, v := range map[string]string{"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Version": "13",
		"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==", "Sec-WebSocket-Protocol": "mqtt", "Origin": "https://evil.example"} {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

// Plain MQTT clients log in with workspace id + token; they cannot publish, subscribe outside
// their workspace, or use multi-level wildcards.
func TestTCPClients(t *testing.T) {
	b := broker(t)
	addr := freeAddr(t)
	if err := b.ListenTCP(addr); err != nil {
		t.Fatal(err)
	}
	opts := func(user, pass string) *paho.ClientOptions {
		return paho.NewClientOptions().AddBroker("tcp://" + addr).SetClientID("cli").SetUsername(user).SetPassword(pass)
	}
	for _, bad := range [][2]string{{ws, "wrong"}, {other, "ann-token"}} {
		c := paho.NewClient(opts(bad[0], bad[1]).SetAutoReconnect(false).SetConnectTimeout(3 * time.Second))
		if tok := c.Connect(); tok.WaitTimeout(3*time.Second) && tok.Error() == nil {
			t.Fatalf("login %v accepted", bad)
		}
	}
	got, c := subscribe(t, opts(ws, "ann-token"), live.Topic(ws, "KYB"))
	for _, filter := range []string{live.Topic(other, "+"), "kyber/#", "#", "kyber/" + ws + "/projects/#", live.Topic(ws, "")} {
		tok := c.Subscribe(filter, 0, nil)
		tok.WaitTimeout(3 * time.Second)
		if st, ok := tok.(*paho.SubscribeToken); !ok || st.Result()[filter] < 0x80 {
			t.Errorf("subscribed to %q", filter)
		}
	}
	c.Publish(live.Topic(ws, "KYB"), 0, false, `{"project":"KYB","event":"forged"}`).WaitTimeout(time.Second)
	b.Publish(live.Change{Workspace: ws, Project: "KYB", Event: "sprint.started"})
	if m := got.next(t); !strings.Contains(string(m.Payload()), "sprint.started") {
		t.Fatalf("got %s (a client publish must be dropped)", m.Payload())
	}
	got.none(t)
	if err := b.ListenTCP(addr); err == nil {
		t.Fatal("second listener with the same id accepted")
	}
}

type failing struct{}

func (failing) Broadcast(context.Context, live.Change) error { return errors.New("db down") }

func TestOnEvent(t *testing.T) {
	var logged strings.Builder
	log := slog.New(slog.NewTextHandler(&logged, nil))
	if err := live.OnEvent(failing{}, log)(context.Background(), outbox.Message{Name: "e", Payload: []byte(`{"key":"KYB-1"}`)}); err != nil || !strings.Contains(logged.String(), "db down") {
		t.Fatalf("a failed hint must not fail the event: %v %q", err, logged.String())
	}
	if err := live.OnEvent(failing{}, log)(context.Background(), outbox.Message{Name: "e", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
}

// S88: chat, calls and whiteboards reach a user on their own topic; nobody else receives it, even with
// wildcards, and nobody may subscribe to another user's topic by name.
func TestUserTopics(t *testing.T) {
	b := broker(t)
	addr := freeAddr(t)
	if err := b.ListenTCP(addr); err != nil {
		t.Fatal(err)
	}
	opts := func(pass string) *paho.ClientOptions {
		return paho.NewClientOptions().AddBroker("tcp://" + addr).SetClientID("cli").SetUsername(ws).SetPassword(pass)
	}
	filter := live.UserTopic("+", "+")
	ann, _ := subscribe(t, opts("ann-token"), filter)
	ben, benc := subscribe(t, opts("ben-token"), filter)
	if got := b.Online(ws, []string{"ben", "carl", "ann"}); strings.Join(got, ",") != "ben,ann" {
		t.Fatalf("online = %v", got)
	}

	b.PublishTo(ws, []string{"ann"}, "chat.message", []byte(`{"body":"hi"}`))
	b.PublishTo(other, []string{"ben"}, "chat.message", []byte(`{}`))
	m := ann.next(t)
	if m.Topic() != live.UserTopic(ws, "ann") || string(m.Payload()) != `{"type":"chat.message","data":{"body":"hi"}}` {
		t.Fatalf("event = %s %s", m.Topic(), m.Payload())
	}
	ann.none(t)
	ben.none(t)

	for _, f := range []string{live.UserTopic(ws, "ann"), live.UserTopic(other, "ben"), "kyber/" + ws + "/users/#", live.UserTopic(ws, "")} {
		tok := benc.Subscribe(f, 0, nil)
		tok.WaitTimeout(3 * time.Second)
		if st, ok := tok.(*paho.SubscribeToken); !ok || st.Result()[f] < 0x80 {
			t.Errorf("ben subscribed to %q", f)
		}
	}
	if got, _ := subscribe(t, opts("ben-token"), live.UserTopic(ws, "ben")); got == nil {
		t.Fatal("own topic refused")
	}
}

// A deactivated user (SCIM) stops receiving events at once and is disconnected, whatever the topic.
func TestDeactivatedUsersAreDisconnected(t *testing.T) {
	var mu sync.Mutex
	gone := map[string]bool{}
	b := brokerWith(t, func(ctx context.Context, user string) bool {
		mu.Lock()
		defer mu.Unlock()
		w, _ := tenant.From(ctx)
		return w == ws && !gone[user]
	})
	addr := freeAddr(t)
	if err := b.ListenTCP(addr); err != nil {
		t.Fatal(err)
	}
	opts := paho.NewClientOptions().AddBroker("tcp://" + addr).SetUsername(ws).SetPassword("ann-token")
	hints, hc := subscribe(t, opts.SetClientID("a"), live.Topic(ws, "KYB"))
	own, oc := subscribe(t, paho.NewClientOptions().AddBroker("tcp://"+addr).SetUsername(ws).SetPassword("ann-token").SetClientID("b"), live.UserTopic(ws, "ann"))
	b.Publish(live.Change{Workspace: ws, Project: "KYB", Event: "issue.created"})
	b.PublishTo(ws, []string{"ann"}, "chat.message", []byte(`{}`))
	hints.next(t)
	own.next(t)

	mu.Lock()
	gone["ann"] = true
	mu.Unlock()
	b.Publish(live.Change{Workspace: ws, Project: "KYB", Event: "issue.changed"})
	b.PublishTo(ws, []string{"ann"}, "chat.message", []byte(`{}`))
	hints.none(t)
	own.none(t)
	for _, c := range []paho.Client{hc, oc} {
		deadline := time.Now().Add(3 * time.Second)
		for c.IsConnectionOpen() && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if c.IsConnectionOpen() {
			t.Fatal("deactivated client still connected")
		}
	}
	if got := b.Online(ws, []string{"ann"}); len(got) != 0 {
		t.Fatalf("online = %v", got)
	}
}
