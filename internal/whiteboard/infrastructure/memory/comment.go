package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

// Comments stores element comments in memory, with their published events.
type Comments struct {
	mu     sync.Mutex
	all    []domain.Comment
	outbox []domain.CommentAdded
	outAt  []outbox.Meta
}

func NewComments() *Comments { return &Comments{} }

func (s *Comments) Outbox() []domain.CommentAdded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.outbox)
}

func (s *Comments) OutboxMeta() []outbox.Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.outAt)
}

func (s *Comments) Add(ctx context.Context, c *domain.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := outbox.MetaOf(ctx, time.Now())
	for _, e := range c.PullEvents() {
		s.outbox, s.outAt = append(s.outbox, e), append(s.outAt, at)
	}
	s.all = append(s.all, *c)
	return nil
}

func (s *Comments) Remove(_ context.Context, id domain.CommentID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = slices.DeleteFunc(s.all, func(c domain.Comment) bool { return c.ID == id })
	return nil
}

func (s *Comments) ByID(_ context.Context, id domain.CommentID) (*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.all {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, domain.ErrCommentMissing
}

func (s *Comments) OnBoard(_ context.Context, b domain.WhiteboardID) ([]*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Comment{}
	for _, c := range s.all {
		if c.Board == b {
			x := c
			out = append(out, &x)
		}
	}
	return out, nil
}
