// Package memory is the in-memory meeting store (tests, dev mode); its events feed the outbox relay.
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type Repository struct {
	mu       sync.Mutex
	meetings map[domain.MeetingID]domain.Meeting
	outbox   []domain.Event
	outAt    []outbox.Meta
}

func NewRepository() *Repository { return &Repository{meetings: map[domain.MeetingID]domain.Meeting{}} }

func clone(m domain.Meeting) *domain.Meeting {
	m.Attendees = slices.Clone(m.Attendees)
	m.Responses = maps.Clone(m.Responses)
	if m.Responses == nil {
		m.Responses = map[domain.UserID]domain.Response{}
	}
	return &m
}

// record appends the meeting's raised events to the outbox log (caller holds mu).
func (r *Repository) record(ctx context.Context, m *domain.Meeting) {
	meta := outbox.MetaOf(ctx, time.Now())
	for _, e := range m.PullEvents() {
		r.outbox = append(r.outbox, e)
		r.outAt = append(r.outAt, meta)
	}
}

func (r *Repository) Create(ctx context.Context, m *domain.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(ctx, m)
	r.meetings[m.ID] = *clone(*m)
	return nil
}

func (r *Repository) Cancel(ctx context.Context, m *domain.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.meetings[m.ID]; !ok {
		return domain.ErrMeetingNotFound
	}
	delete(r.meetings, m.ID)
	r.record(ctx, m)
	return nil
}

func (r *Repository) ByID(_ context.Context, id domain.MeetingID) (*domain.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.meetings[id]
	if !ok {
		return nil, domain.ErrMeetingNotFound
	}
	return clone(m), nil
}

func (r *Repository) ForUser(_ context.Context, u domain.UserID, from, to time.Time) ([]*domain.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Meeting
	for _, m := range r.meetings {
		if m.Blocks(u, from, to) {
			out = append(out, clone(m))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Meeting) int { return a.Start.Compare(b.Start) })
	return out, nil
}

func (r *Repository) Respond(_ context.Context, m *domain.Meeting, u domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.meetings[m.ID]
	if !ok {
		return domain.ErrMeetingNotFound
	}
	c := clone(stored)
	c.Responses[u] = m.Responses[u]
	r.meetings[m.ID] = *c
	return nil
}

func (r *Repository) DueForReminder(_ context.Context, from, to time.Time) ([]*domain.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Meeting
	for _, m := range r.meetings {
		if !m.Reminded && !m.Start.Before(from) && m.Start.Before(to) {
			out = append(out, clone(m))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Meeting) int { return a.Start.Compare(b.Start) })
	return out, nil
}

func (r *Repository) SaveReminder(ctx context.Context, m *domain.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.meetings[m.ID]
	if !ok || stored.Reminded {
		return domain.ErrAlreadyReminded
	}
	stored.Reminded = true
	r.meetings[m.ID] = stored
	r.record(ctx, m)
	return nil
}

// Outbox is the append-only event log the relay reads.
func (r *Repository) Outbox() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outbox)
}

// OutboxMeta is the envelope of Outbox(), index by index.
func (r *Repository) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outAt)
}
