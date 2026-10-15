// Package memory keeps SSO connections in memory (dev mode and tests).
package memory

import (
	"context"
	"sort"
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
	return &c, nil
}

func (s *Store) Save(_ context.Context, c *domain.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[c.ID] = *c
	return nil
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
