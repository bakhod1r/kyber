// Package memory keeps SSO connections in memory (dev mode and tests).
package memory

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

type Store struct {
	mu   sync.Mutex
	byID map[domain.ConnectionID]domain.Connection
}

func New() *Store { return &Store{byID: map[domain.ConnectionID]domain.Connection{}} }

func (s *Store) All(context.Context) ([]domain.Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Connection, 0, len(s.byID))
	for _, c := range s.byID {
		c.Domains = slices.Clone(c.Domains)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Store) ByID(_ context.Context, id domain.ConnectionID) (*domain.Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.byID[id]
	if !ok {
		return nil, domain.ErrConnectionNotFound
	}
	c.Domains = slices.Clone(c.Domains)
	return &c, nil
}

func (s *Store) Save(_ context.Context, c *domain.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, other := range s.byID {
		for _, d := range other.Domains {
			if id != c.ID && slices.ContainsFunc(c.Domains, func(x domain.Domain) bool { return x.Name == d.Name }) {
				return domain.ErrDomainTaken
			}
		}
	}
	cp := *c
	cp.Domains = slices.Clone(c.Domains)
	s.byID[c.ID] = cp
	return nil
}

func (s *Store) RequiredBy(_ context.Context, workspace string) ([]domain.Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Connection{}
	for _, c := range s.byID {
		if c.Requires(workspace) {
			c.Domains = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) ByDomain(_ context.Context, name string) (*domain.Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.ToLower(name)
	for _, c := range s.byID {
		for _, d := range c.Domains {
			if d.Name == name {
				c.Domains = slices.Clone(c.Domains)
				return &c, nil
			}
		}
	}
	return nil, domain.ErrConnectionNotFound
}

func (s *Store) Delete(_ context.Context, id domain.ConnectionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return domain.ErrConnectionNotFound
	}
	delete(s.byID, id)
	return nil
}
