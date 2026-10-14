package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type savedKey struct {
	ws   domain.WorkspaceID
	user domain.UserID
	msg  domain.MessageID
}

// Saved stores saved messages next to r's messages (for reminder excerpts).
type Saved struct {
	r      *Repository
	mu     sync.Mutex
	all    map[savedKey]domain.Saved
	outbox []domain.ReminderDue
	outAt  []outbox.Meta
}

func NewSaved(r *Repository) *Saved { return &Saved{r: r, all: map[savedKey]domain.Saved{}} }

func (s *Saved) Outbox() []domain.ReminderDue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.outbox)
}

func (s *Saved) OutboxMeta() []outbox.Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.outAt)
}

func keyOf(x *domain.Saved) savedKey { return savedKey{x.Workspace, x.User, x.Message} }

func (s *Saved) Put(_ context.Context, x *domain.Saved) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *x
	c.Reminded = false
	c.PullEvents()
	s.all[keyOf(x)] = c
	return nil
}

func (s *Saved) Remove(_ context.Context, ws domain.WorkspaceID, u domain.UserID, m domain.MessageID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.all, savedKey{ws, u, m})
	return nil
}

func (s *Saved) For(_ context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Saved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.Saved
	for k, x := range s.all {
		if k.ws == ws && k.user == u {
			c := x
			out = append(out, &c)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Saved) int { return b.SavedAt.Compare(a.SavedAt) })
	return out, nil
}

func (s *Saved) Due(_ context.Context, now time.Time) ([]*domain.Saved, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.Saved
	for _, x := range s.all {
		if !x.Reminded && !x.RemindAt.IsZero() && !x.RemindAt.After(now) {
			c := x
			body := ""
			if m, err := (&Messages{r: s.r}).ByID(context.Background(), x.Message); err == nil {
				body = m.Body
			}
			out = append(out, c.WithExcerpt(body))
		}
	}
	return out, nil
}

func (s *Saved) MarkReminded(ctx context.Context, x *domain.Saved) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.all[keyOf(x)]
	if !ok || cur.Reminded {
		return domain.ErrAlreadyReminded
	}
	cur.Reminded = true
	s.all[keyOf(x)] = cur
	at := outbox.MetaOf(ctx, time.Now())
	at.Workspace = string(x.Workspace)
	for _, e := range x.PullEvents() {
		s.outbox, s.outAt = append(s.outbox, e), append(s.outAt, at)
	}
	return nil
}
