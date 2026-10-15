// Package realtime fans events out to the browser tabs of users (ADR-0009): a Hub of
// subscriptions per workspace and user, served as Server-Sent Events. Recent events stay in a
// ring so a reconnecting tab resumes from its Last-Event-ID. The Hub is in-process; across
// replicas a LISTEN/NOTIFY or Redis feed calls Publish on every node.
package realtime

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/platform/presence"
)

// Buffer is how many undelivered events a subscription holds before it is closed (the
// client then reconnects and replays from the ring).
const Buffer = 64

type Event struct {
	ID   uint64
	Type string
	Data json.RawMessage
}

type addressed struct {
	ws    string
	users []string
	Event
}

type sub struct {
	ws, user string
	ch       chan Event
	closed   bool
}

// Sink is another way events reach users (S88: the MQTT broker's per-user topics); it also knows who
// is connected through it.
type Sink interface {
	PublishTo(ws string, users []string, typ string, data []byte)
	Online(ws string, users []string) []string
}

// Presence answers who is online on any replica and wants to know when this replica's set changes
// (S105, presence.Roster).
type Presence interface {
	Online(ctx context.Context, ws string, users []string) ([]string, error)
	Changed()
}

// seater is a sink that can list its connected users (the MQTT broker).
type seater interface{ Seats() []presence.Seat }

type Hub struct {
	sink   Sink
	roster Presence
	mu     sync.Mutex
	next   uint64
	ring   []addressed
	size   int
	subs   map[*sub]struct{}
}

// NewHub keeps the last ring events for replay.
func NewHub(ring int) *Hub { return &Hub{size: ring, subs: map[*sub]struct{}{}} }

func (a addressed) to(ws, user string) bool { return a.ws == ws && slices.Contains(a.users, user) }

// Subscribe opens a feed for user in ws, first replaying ring events after lastID. The stop
// function is idempotent.
func (h *Hub) Subscribe(ws, user string, lastID uint64) (<-chan Event, func()) {
	defer h.changed() // after the lock is released
	h.mu.Lock()
	defer h.mu.Unlock()
	s := &sub{ws: ws, user: user, ch: make(chan Event, Buffer)}
	if lastID > 0 {
		for _, e := range h.ring {
			if e.ID > lastID && e.to(ws, user) {
				h.deliver(s, e.Event)
			}
		}
	}
	h.subs[s] = struct{}{}
	return s.ch, func() {
		h.mu.Lock()
		was := !s.closed
		h.close(s)
		h.mu.Unlock()
		if was {
			h.changed()
		}
	}
}

func (h *Hub) close(s *sub) {
	if !s.closed {
		s.closed = true
		close(s.ch)
		delete(h.subs, s)
	}
}

// deliver never blocks: a subscriber too slow to keep up is closed.
func (h *Hub) deliver(s *sub, e Event) {
	if s.closed {
		return
	}
	select {
	case s.ch <- e:
	default:
		h.close(s)
	}
}

// Track answers presence through p and tells it when streams open or close. Call it before use.
func (h *Hub) Track(p Presence) { h.roster = p }

func (h *Hub) changed() {
	if h.roster != nil {
		h.roster.Changed()
	}
}

// Seats lists who is connected to this replica: open streams and the sink's clients.
func (h *Hub) Seats() []presence.Seat {
	var out []presence.Seat
	if s, ok := h.sink.(seater); ok {
		out = append(out, s.Seats()...)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if !s.closed {
			out = append(out, presence.Seat{Workspace: s.ws, User: s.user})
		}
	}
	return out
}

// Forward also hands every published event to sink. Call it before the hub is used.
func (h *Hub) Forward(sink Sink) { h.sink = sink }

// Publish sends an event (data as JSON) to every open tab of the given users in ws.
func (h *Hub) Publish(ws string, users []string, typ string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if h.sink != nil {
		h.sink.PublishTo(ws, users, typ, b)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	e := addressed{ws: ws, users: slices.Clone(users), Event: Event{ID: h.next, Type: typ, Data: b}}
	if h.size > 0 {
		if len(h.ring) == h.size {
			h.ring = slices.Delete(h.ring, 0, 1)
		}
		h.ring = append(h.ring, e)
	}
	for s := range h.subs {
		if e.to(s.ws, s.user) {
			h.deliver(s, e.Event)
		}
	}
	return nil
}

// Online lists which of users have an open stream in ws (in the given order).
func (h *Hub) Online(ws string, users []string) []string {
	on := map[string]bool{}
	if h.sink != nil {
		for _, u := range h.sink.Online(ws, users) {
			on[u] = true
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if s.ws == ws {
			on[s.user] = true
		}
	}
	out := []string{}
	for _, u := range users {
		if on[u] && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out
}

// Connections counts the streams open on this replica (a metric).
func (h *Hub) Connections() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
