package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// Scheduled stores scheduled messages in memory.
type Scheduled struct {
	mu  sync.Mutex
	all map[domain.ScheduledID]domain.Scheduled
}

func NewScheduled() *Scheduled { return &Scheduled{all: map[domain.ScheduledID]domain.Scheduled{}} }

func (s *Scheduled) Add(_ context.Context, x *domain.Scheduled) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all[x.ID] = *x
	return nil
}

func (s *Scheduled) Update(_ context.Context, x *domain.Scheduled) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.all[x.ID]; !ok {
		return domain.ErrScheduledNotFound
	}
	s.all[x.ID] = *x
	return nil
}

func (s *Scheduled) Delete(_ context.Context, id domain.ScheduledID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.all, id)
	return nil
}

func (s *Scheduled) ByID(_ context.Context, id domain.ScheduledID) (*domain.Scheduled, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, ok := s.all[id]
	if !ok {
		return nil, domain.ErrScheduledNotFound
	}
	return &x, nil
}

func (s *Scheduled) list(keep func(domain.Scheduled) bool) []*domain.Scheduled {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Scheduled{}
	for _, x := range s.all {
		if keep(x) {
			c := x
			out = append(out, &c)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Scheduled) int {
		if c := a.SendAt.Compare(b.SendAt); c != 0 {
			return c
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return out
}

func (s *Scheduled) Mine(_ context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Scheduled, error) {
	return s.list(func(x domain.Scheduled) bool { return x.Workspace == ws && x.Author == u }), nil
}

func (s *Scheduled) Due(_ context.Context, now time.Time) ([]*domain.Scheduled, error) {
	return s.list(func(x domain.Scheduled) bool { return !x.SendAt.After(now) }), nil
}

// Reminders stores personal reminders in memory.
type Reminders struct {
	mu     sync.Mutex
	all    map[domain.ReminderID]domain.Reminder
	outbox []domain.ReminderDue
	outAt  []outbox.Meta
}

func NewReminders() *Reminders { return &Reminders{all: map[domain.ReminderID]domain.Reminder{}} }

func (r *Reminders) Outbox() []domain.ReminderDue {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outbox)
}

func (r *Reminders) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outAt)
}

func (r *Reminders) Add(_ context.Context, x *domain.Reminder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *x
	c.PullEvents()
	r.all[x.ID] = c
	return nil
}

func (r *Reminders) Due(_ context.Context, now time.Time) ([]*domain.Reminder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*domain.Reminder{}
	for _, x := range r.all {
		if !x.Reminded && !x.At.After(now) {
			c := x
			out = append(out, &c)
		}
	}
	return out, nil
}

func (r *Reminders) MarkReminded(ctx context.Context, x *domain.Reminder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.all[x.ID]
	if !ok || cur.Reminded {
		return domain.ErrAlreadyReminded
	}
	cur.Reminded = true
	r.all[x.ID] = cur
	at := outbox.MetaOf(ctx, time.Now())
	at.Workspace = string(x.Workspace)
	for _, e := range x.PullEvents() {
		r.outbox, r.outAt = append(r.outbox, e), append(r.outAt, at)
	}
	return nil
}
