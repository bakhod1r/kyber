// Package memory holds ssosync state in process (dev mode and tests).
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

type Store struct {
	mu     sync.Mutex
	maps   map[domain.MappingID]domain.Mapping
	grants map[string]map[domain.Target]string
}

func New() *Store {
	return &Store{maps: map[domain.MappingID]domain.Mapping{}, grants: map[string]map[domain.Target]string{}}
}

func (s *Store) All(context.Context) ([]domain.Mapping, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Mapping, 0, len(s.maps))
	for _, m := range s.maps {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) Create(_ context.Context, m *domain.Mapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maps[m.ID] = *m
	return nil
}

func (s *Store) Delete(_ context.Context, id domain.MappingID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.maps[id]; !ok {
		return domain.ErrMappingNotFound
	}
	delete(s.maps, id)
	return nil
}

func (s *Store) Of(_ context.Context, user string) ([]domain.Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Grant
	for t, r := range s.grants[user] {
		out = append(out, domain.Grant{User: user, Target: t, Role: r})
	}
	return out, nil
}

func (s *Store) Save(_ context.Context, g domain.Grant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.grants[g.User] == nil {
		s.grants[g.User] = map[domain.Target]string{}
	}
	s.grants[g.User][g.Target] = g.Role
	return nil
}

func (s *Store) Remove(_ context.Context, user string, t domain.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.grants[user], t)
	return nil
}

var (
	_ domain.Mappings = (*Store)(nil)
	_ domain.Grants   = (*Store)(nil)
)
