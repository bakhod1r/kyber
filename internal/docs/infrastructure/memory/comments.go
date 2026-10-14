package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type stored struct {
	ws string
	c  domain.Comment
}

// Comments keeps paragraph comments in memory, with their published events; a page's comments go with it.
type Comments struct {
	pages  *Repository
	mu     sync.Mutex
	all    []stored
	outbox []domain.CommentAdded
	outAt  []outbox.Meta
}

func NewComments(pages *Repository) *Comments { return &Comments{pages: pages} }

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
	s.all = append(s.all, stored{ws: tenant.Workspace(ctx), c: *c})
	return nil
}

func (s *Comments) Remove(ctx context.Context, id domain.CommentID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = slices.DeleteFunc(s.all, func(x stored) bool { return x.c.ID == id && x.ws == tenant.Workspace(ctx) })
	return nil
}

// live reports whether the comment's page still exists (deleting a page deletes its comments).
func (s *Comments) live(ctx context.Context, x stored) bool {
	if x.ws != tenant.Workspace(ctx) {
		return false
	}
	_, err := s.pages.ByID(ctx, x.c.Page)
	return err == nil
}

func (s *Comments) ByID(ctx context.Context, id domain.CommentID) (*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.all {
		if x.c.ID == id && s.live(ctx, x) {
			c := x.c
			return &c, nil
		}
	}
	return nil, domain.ErrCommentMissing
}

func (s *Comments) OnPage(ctx context.Context, p domain.PageID) ([]*domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Comment{}
	for _, x := range s.all {
		if x.c.Page == p && s.live(ctx, x) {
			c := x.c
			out = append(out, &c)
		}
	}
	slices.SortStableFunc(out, func(a, b *domain.Comment) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}
