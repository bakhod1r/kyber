// Package memory is the in-memory focus-session store (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/focus/domain"
)

type Repository struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
}

func NewRepository() *Repository { return &Repository{sessions: map[string]domain.Session{}} }

func (r *Repository) Create(_ context.Context, s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.sessions {
		if x.User == s.User && x.State.Active() {
			return domain.ErrAlreadyRunning
		}
	}
	r.sessions[s.ID] = *s
	return nil
}

func (r *Repository) Save(_ context.Context, s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = *s
	return nil
}

func (r *Repository) Active(_ context.Context, user string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.sessions {
		if x.User == user && x.State.Active() {
			return &x, nil
		}
	}
	return nil, domain.ErrSessionNotFound
}

func (r *Repository) list(keep func(domain.Session) bool) []*domain.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Session
	for _, x := range r.sessions {
		if keep(x) {
			out = append(out, &x)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Session) int { return b.StartedAt.Compare(a.StartedAt) })
	return out
}

func (r *Repository) ForIssue(_ context.Context, workspace, issue string) ([]*domain.Session, error) {
	return r.list(func(x domain.Session) bool { return x.Workspace == workspace && x.Issue == issue }), nil
}

func (r *Repository) ForUser(_ context.Context, user string, from, to time.Time) ([]*domain.Session, error) {
	return r.list(func(x domain.Session) bool {
		return x.User == user && !x.StartedAt.Before(from) && x.StartedAt.Before(to)
	}), nil
}
