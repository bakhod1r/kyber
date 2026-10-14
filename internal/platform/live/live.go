// Package live pushes "something changed" hints to browsers over Server-Sent Events. A hint names
// only the project and the event (never titles, keys or other content), and is sent only to
// subscribers who may browse that project; clients then refetch through the authorized API.
package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Change is one hint: an event happened in a project of a workspace.
type Change struct {
	Workspace string `json:"-"`
	Project   string `json:"project"`
	Event     string `json:"event"`
}

// FromMessage reads the project of an outbox message from its payload ("project", or the
// project part of an issue "key"); ok is false for events outside any project.
func FromMessage(m outbox.Message) (Change, bool) {
	var p struct {
		Project string `json:"project"`
		Key     string `json:"key"`
	}
	if json.Unmarshal(m.Payload, &p) != nil {
		return Change{}, false
	}
	project := p.Project
	if i := strings.LastIndexByte(p.Key, '-'); project == "" && i > 0 {
		project = p.Key[:i]
	}
	return Change{Workspace: m.Workspace, Project: project, Event: m.Name}, project != ""
}

// buffer is how many hints a slow subscriber may fall behind before hints are dropped (it is
// told to resync instead).
const buffer = 64

type subscriber struct {
	workspace string
	ch        chan Change
	mu        sync.Mutex
	lagged    bool
}

// Hub fans changes out to this process's subscribers.
type Hub struct {
	mu   sync.Mutex
	subs map[*subscriber]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

// Subscribe receives the workspace's changes until cancel is called.
func (h *Hub) subscribe(workspace string) (*subscriber, func()) {
	s := &subscriber{workspace: workspace, ch: make(chan Change, buffer)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s, func() {
		h.mu.Lock()
		delete(h.subs, s)
		h.mu.Unlock()
	}
}

// Publish hands c to the workspace's subscribers without blocking; a full subscriber is marked
// as lagged and will be told to resync.
func (h *Hub) Publish(c Change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if s.workspace != c.Workspace {
			continue
		}
		select {
		case s.ch <- c:
		default:
			s.mu.Lock()
			s.lagged = true
			s.mu.Unlock()
		}
	}
}

// Subscribers counts open streams (for tests and metrics).
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// CanBrowse reports whether user may see project (the composition root asks the Project context).
type CanBrowse func(ctx context.Context, user, project string) bool

// Handler streams the caller's workspace hints as SSE: "change" events with {project, event},
// "resync" when hints were dropped, and a comment every heartbeat to keep proxies from closing it.
func (h *Hub) Handler(can CanBrowse, heartbeat time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, workspace := auth.Actor(ctx), tenant.Workspace(ctx)
		rc := http.NewResponseController(w)
		// The stream outlives the server's write timeout.
		if err := rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		s, cancel := h.subscribe(workspace)
		defer cancel()
		send := func(format string, args ...any) bool {
			if _, err := fmt.Fprintf(w, format, args...); err != nil {
				return false
			}
			return rc.Flush() == nil
		}
		if !send("retry: 3000\n: connected\n\n") {
			return
		}
		tick := time.NewTicker(heartbeat)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if !send(": ping\n\n") {
					return
				}
			case c := <-s.ch:
				s.mu.Lock()
				lagged := s.lagged
				s.lagged = false
				s.mu.Unlock()
				if lagged && !send("event: resync\ndata: {}\n\n") {
					return
				}
				if !can(ctx, user, c.Project) {
					continue
				}
				data, _ := json.Marshal(c)
				if !send("event: change\ndata: %s\n\n", data) {
					return
				}
			}
		}
	})
}

// Hint is a server-sent event for another stream: Type "change" (Data {project, event}) or "resync".
type Hint struct {
	Type string
	Data []byte
}

// Follow delivers the workspace's hints that user may see until ctx ends (S83: the chat stream carries them, so
// a browser tab needs one stream).
func (h *Hub) Follow(ctx context.Context, user, workspace string, can CanBrowse) <-chan Hint {
	out := make(chan Hint, buffer)
	s, cancel := h.subscribe(workspace)
	go func() {
		defer cancel()
		defer close(out)
		emit := func(hint Hint) bool {
			select {
			case out <- hint:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case c := <-s.ch:
				s.mu.Lock()
				lagged := s.lagged
				s.lagged = false
				s.mu.Unlock()
				if lagged && !emit(Hint{Type: "resync", Data: []byte("{}")}) {
					return
				}
				if !can(ctx, user, c.Project) {
					continue
				}
				data, _ := json.Marshal(c)
				if !emit(Hint{Type: "change", Data: data}) {
					return
				}
			}
		}
	}()
	return out
}

// Broadcaster delivers a change to the subscribers of every replica.
type Broadcaster interface {
	Broadcast(ctx context.Context, c Change) error
}

// Broadcast publishes to this process only (a single replica, or dev mode).
func (h *Hub) Broadcast(_ context.Context, c Change) error {
	h.Publish(c)
	return nil
}

// OnEvent is the outbox handler that turns project events into hints. Hints are best effort: a
// failure is logged, never retried, so it cannot hold up or dead-letter the event.
func OnEvent(b Broadcaster, log interface {
	WarnContext(ctx context.Context, msg string, args ...any)
}) outbox.Handler {
	return func(ctx context.Context, m outbox.Message) error {
		if c, ok := FromMessage(m); ok {
			if err := b.Broadcast(ctx, c); err != nil {
				log.WarnContext(ctx, "live: broadcast failed", "event", m.Name, "err", err)
			}
		}
		return nil
	}
}
