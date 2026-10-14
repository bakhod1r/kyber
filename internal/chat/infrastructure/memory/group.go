package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

// Groups stores user groups in memory.
type Groups struct {
	mu  sync.Mutex
	all map[domain.GroupID]domain.Group
}

func NewGroups() *Groups { return &Groups{all: map[domain.GroupID]domain.Group{}} }

func cloneGroup(g domain.Group) *domain.Group {
	g.Members = slices.Clone(g.Members)
	return &g
}

func (s *Groups) Save(_ context.Context, g *domain.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.all {
		if o.ID != g.ID && o.Workspace == g.Workspace && o.Handle == g.Handle {
			return domain.ErrHandleTaken
		}
	}
	s.all[g.ID] = *cloneGroup(*g)
	return nil
}

func (s *Groups) Delete(_ context.Context, id domain.GroupID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.all, id)
	return nil
}

func (s *Groups) ByID(_ context.Context, id domain.GroupID) (*domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.all[id]
	if !ok {
		return nil, domain.ErrGroupNotFound
	}
	return cloneGroup(g), nil
}

func (s *Groups) ByHandle(_ context.Context, ws domain.WorkspaceID, handle string) (*domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.all {
		if g.Workspace == ws && g.Handle == handle {
			return cloneGroup(g), nil
		}
	}
	return nil, domain.ErrGroupNotFound
}

func (s *Groups) In(_ context.Context, ws domain.WorkspaceID) ([]*domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Group{}
	for _, g := range s.all {
		if g.Workspace == ws {
			out = append(out, cloneGroup(g))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Group) int { return cmp.Compare(a.Handle, b.Handle) })
	return out, nil
}
