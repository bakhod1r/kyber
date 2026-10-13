// Package outbox relays domain events from the transactional outbox to in-process
// handlers: at-least-once, in order within a batch, retried until a handler succeeds.
// Handlers must therefore be idempotent (key their effects on Message.ID).
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/bakhod1r/jitterx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/bakhod1r/kyber/internal/platform/otel"
)

// Event is a domain event (any context's) as stored in the outbox.
type Event interface{ EventName() string }

// SchemaVersion is the envelope and payload version written with new events; handlers can
// tell older stored events apart when an event's shape changes.
const SchemaVersion = 1

// Message is one stored event with its envelope.
type Message struct {
	ID        int64
	Name      string
	Payload   []byte
	At        time.Time // when the event was recorded
	Workspace string    // the tenant the event belongs to; handlers run scoped to it
	Actor     string    // the user whose request recorded it ("" = the system)
	Trace     string    // W3C traceparent of that request ("" = untraced)
	Version   int       // SchemaVersion at recording time
	Attempts  int       // failed deliveries so far
	LastError string    // the latest delivery failure
}

// ErrNotDead: Retry was asked for a message that is not a dead letter.
var ErrNotDead = errors.New("outbox message is not a dead letter")

// DeadLetter tells the store to park a message (dead_at) instead of retrying it.
type DeadLetter struct{ Err error }

func (d *DeadLetter) Error() string { return "dead letter: " + d.Err.Error() }
func (d *DeadLetter) Unwrap() error { return d.Err }

// Store claims unpublished messages in order and marks those fn accepted as published.
// It stops at the first failure so later events never overtake an earlier one.
type Store interface {
	Process(ctx context.Context, limit int, fn func(Message) error) (published int, err error)
}

type Handler func(ctx context.Context, m Message) error

type Relay struct {
	store       Store
	log         *slog.Logger
	maxAttempts int
	mu          sync.RWMutex
	handlers    map[string][]Handler
	all         []Handler
}

// DefaultMaxAttempts is how often a message may fail before it becomes a dead letter.
const DefaultMaxAttempts = 10

type Option func(*Relay)

// MaxAttempts sets how often a message may fail before it is parked as a dead letter (at least 1).
func MaxAttempts(n int) Option { return func(r *Relay) { r.maxAttempts = max(n, 1) } }

func NewRelay(store Store, log *slog.Logger, opts ...Option) *Relay {
	r := &Relay{store: store, log: log, maxAttempts: DefaultMaxAttempts, handlers: map[string][]Handler{}}
	for _, o := range opts {
		o(r)
	}
	return r
}

// HandleAll subscribes h to every event, after the handlers of its name.
func (r *Relay) HandleAll(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all = append(r.all, h)
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
		hs := append(slices.Clone(r.handlers[m.Name]), r.all...)
		r.mu.RUnlock()
		hctx := ctx
		if m.Workspace != "" {
			hctx = tenant.With(ctx, m.Workspace)
		}
		if m.Trace != "" { // continue the trace of the request that recorded the event
			hctx = gotel.GetTextMapPropagator().Extract(hctx, propagation.MapCarrier{"traceparent": m.Trace})
		}
		hctx, span := otel.Tracer().Start(hctx, "outbox "+m.Name, trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attribute.Int64("outbox.id", m.ID), attribute.Int("outbox.handlers", len(hs))))
		defer span.End()
		for _, h := range hs {
			if err := h(hctx, m); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				err = fmt.Errorf("outbox %d %s: %w", m.ID, m.Name, err)
				if m.Attempts+1 >= r.maxAttempts {
					r.log.ErrorContext(hctx, "outbox: dead-lettered after repeated failures", "id", m.ID, "event", m.Name,
						"attempts", m.Attempts+1, "err", err)
					return &DeadLetter{Err: err}
				}
				return err
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

// Meta is the envelope the outbox table stores next to an event.
type Meta struct {
	At        time.Time
	Workspace string
	Actor     string
	Trace     string
}

// MetaOf is the envelope of an event recorded at at by the request of ctx: its workspace,
// actor and trace context.
func MetaOf(ctx context.Context, at time.Time) Meta {
	carrier := propagation.MapCarrier{}
	gotel.GetTextMapPropagator().Inject(ctx, carrier)
	return Meta{At: at, Workspace: tenant.Workspace(ctx), Actor: auth.Actor(ctx), Trace: carrier["traceparent"]}
}

// Source is an in-memory repository's full, append-only event log with the time and
// workspace of each event (like the outbox table's created_at and workspace_id).
type Source func() ([]Event, []Meta)

// MemoryStore relays events kept by in-memory repositories (dev mode and tests).
type MemoryStore struct {
	mu      sync.Mutex
	sources []Source
	cursors []int
	nextID  int64
	pending []Message
	dead    []Message
}

func NewMemoryStore(sources ...Source) *MemoryStore {
	return &MemoryStore{sources: sources, cursors: make([]int, len(sources))}
}

func (s *MemoryStore) Process(_ context.Context, limit int, fn func(Message) error) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, src := range s.sources {
		events, at := src()
		for j, e := range events[s.cursors[i]:] {
			payload, err := json.Marshal(e)
			if err != nil {
				return 0, err
			}
			s.nextID++
			m := at[s.cursors[i]+j]
			s.pending = append(s.pending, Message{ID: s.nextID, Name: e.EventName(), Payload: payload, At: m.At, Workspace: m.Workspace,
				Actor: m.Actor, Trace: m.Trace, Version: SchemaVersion})
		}
		s.cursors[i] = len(events)
	}
	done, i := 0, 0
	for i < len(s.pending) && done < limit {
		m := s.pending[i]
		err := fn(m)
		var dl *DeadLetter
		switch {
		case err == nil:
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			done++
		case errors.As(err, &dl):
			m.Attempts, m.LastError = m.Attempts+1, err.Error()
			s.dead = append(s.dead, m)
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
		default:
			s.pending[i].Attempts, s.pending[i].LastError = m.Attempts+1, err.Error()
			return done, err
		}
	}
	return done, nil
}

// DeadLetters lists the parked messages, oldest first.
func (s *MemoryStore) DeadLetters() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.dead)
}

// Retry puts a dead letter back at the head of the queue with its attempts reset.
func (s *MemoryStore) Retry(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.dead, func(m Message) bool { return m.ID == id })
	if i < 0 {
		return ErrNotDead
	}
	m := s.dead[i]
	m.Attempts, m.LastError = 0, ""
	s.dead = slices.Delete(s.dead, i, i+1)
	s.pending = append([]Message{m}, s.pending...)
	return nil
}
