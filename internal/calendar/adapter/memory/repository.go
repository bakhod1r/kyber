// Package memory is the in-memory meeting store (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
)

type Repository struct {
	mu       sync.Mutex
	meetings map[domain.MeetingID]domain.Meeting
}

func NewRepository() *Repository { return &Repository{meetings: map[domain.MeetingID]domain.Meeting{}} }

func (r *Repository) Create(_ context.Context, m *domain.Meeting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *m
	c.Attendees = slices.Clone(m.Attendees)
	r.meetings[m.ID] = c
	return nil
}

func (r *Repository) Delete(_ context.Context, id domain.MeetingID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.meetings[id]; !ok {
		return domain.ErrMeetingNotFound
	}
	delete(r.meetings, id)
	return nil
}

func (r *Repository) ByID(_ context.Context, id domain.MeetingID) (*domain.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.meetings[id]
	if !ok {
		return nil, domain.ErrMeetingNotFound
	}
	m.Attendees = slices.Clone(m.Attendees)
	return &m, nil
}

func (r *Repository) ForUser(_ context.Context, u domain.UserID, from, to time.Time) ([]*domain.Meeting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Meeting
	for _, m := range r.meetings {
		if m.Blocks(u, from, to) {
			c := m
			c.Attendees = slices.Clone(m.Attendees)
			out = append(out, &c)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Meeting) int { return a.Start.Compare(b.Start) })
	return out, nil
}
