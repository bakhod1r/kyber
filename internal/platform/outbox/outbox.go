// Package outbox relays domain events from the transactional outbox to in-process
// handlers: at-least-once, in order within a batch, retried until a handler succeeds.
// Handlers must therefore be idempotent (key their effects on Message.ID).
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bakhod1r/jitterx"
)

// Event is a domain event (any context's) as stored in the outbox.
type Event interface{ EventName() string }

// Message is one stored event.
type Message struct {
	ID      int64
	Name    string
	Payload []byte
}

// Store claims unpublished messages in order and marks those fn accepted as published.
// It stops at the first failure so later events never overtake an earlier one.
type Store interface {
	Process(ctx context.Context, limit int, fn func(Message) error) (published int, err error)
}

type Handler func(ctx context.Context, m Message) error

type Relay struct {
	store    Store
	log      *slog.Logger
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewRelay(store Store, log *slog.Logger) *Relay {
	return &Relay{store: store, log: log, handlers: map[string][]Handler{}}
}

// Handle subscribes h to events named name.
func (r *Relay) Handle(name string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = append(r.handlers[name], h)
}

const batch = 100

// ProcessOnce publishes one batch and returns how many messages were published.
func (r *Relay) ProcessOnce(ctx context.Context) (int, error) {
	return r.store.Process(ctx, batch, func(m Message) error {
		r.mu.RLock()
		hs := r.handlers[m.Name]
		r.mu.RUnlock()
		for _, h := range hs {
			if err := h(ctx, m); err != nil {
				return fmt.Errorf("outbox %d %s: %w", m.ID, m.Name, err)
			}
		}
		return nil
	})
}

// Run drains the outbox on a jittered interval until ctx ends.
func (r *Relay) Run(ctx context.Context, every time.Duration) {
	_ = jitterx.Every(ctx, every, nil, func(ctx context.Context) error {
		for {
			n, err := r.ProcessOnce(ctx)
			if err != nil {
				r.log.WarnContext(ctx, "outbox relay: will retry", "err", err)
				return nil
			}
			if n < batch {
				return nil
			}
		}
	})
}

// MemoryStore relays events kept by in-memory repositories (dev mode and tests).
// Each source returns its full, append-only event log.
type MemoryStore struct {
	mu      sync.Mutex
	sources []func() []Event
	cursors []int
	nextID  int64
	pending []Message
}

func NewMemoryStore(sources ...func() []Event) *MemoryStore {
	return &MemoryStore{sources: sources, cursors: make([]int, len(sources))}
}

func (s *MemoryStore) Process(_ context.Context, limit int, fn func(Message) error) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, src := range s.sources {
		events := src()
		for _, e := range events[s.cursors[i]:] {
			payload, err := json.Marshal(e)
			if err != nil {
				return 0, err
			}
			s.nextID++
			s.pending = append(s.pending, Message{ID: s.nextID, Name: e.EventName(), Payload: payload})
		}
		s.cursors[i] = len(events)
	}
	done := 0
	for done < len(s.pending) && done < limit {
		if err := fn(s.pending[done]); err != nil {
			s.pending = s.pending[done:]
			return done, err
		}
		done++
	}
	s.pending = s.pending[done:]
	return done, nil
}
