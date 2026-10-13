package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

type statusKey struct {
	ws   domain.WorkspaceID
	user domain.UserID
}

// Statuses stores statuses in memory.
type Statuses struct {
	mu  sync.Mutex
	all map[statusKey]domain.Status
}

func NewStatuses() *Statuses { return &Statuses{all: map[statusKey]domain.Status{}} }

func (s *Statuses) Put(_ context.Context, st *domain.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all[statusKey{st.Workspace, st.User}] = *st
	return nil
}

func (s *Statuses) Of(_ context.Context, ws domain.WorkspaceID, users []domain.UserID) ([]*domain.Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Status{}
	for _, u := range slices.Compact(slices.Sorted(slices.Values(users))) {
		if st, ok := s.all[statusKey{ws, u}]; ok {
			out = append(out, &st)
		}
	}
	return out, nil
}
