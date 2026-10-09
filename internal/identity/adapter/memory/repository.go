// Package memory is an in-memory user and session store for tests and dev mode.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

type Repository struct {
	mu       sync.Mutex
	users    map[domain.UserID]domain.User
	sessions map[string]domain.Session
}

func NewRepository() *Repository {
	return &Repository{users: map[domain.UserID]domain.User{}, sessions: map[string]domain.Session{}}
}

func (r *Repository) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.users {
		if x.Email() == u.Email() {
			return domain.ErrEmailTaken
		}
	}
	r.users[u.ID()] = *u
	return nil
}

func (r *Repository) ByEmail(_ context.Context, e domain.Email) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Email() == e {
			return &u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *Repository) ByID(_ context.Context, id domain.UserID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return &u, nil
}

func (r *Repository) CreateSession(_ context.Context, s domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[string(s.TokenHash)] = s
	return nil
}

func (r *Repository) SessionByHash(_ context.Context, hash []byte) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[string(hash)]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return s, nil
}

func (r *Repository) DeleteSession(_ context.Context, hash []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, string(hash))
	return nil
}

func (r *Repository) DeleteExpiredSessions(_ context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for k, s := range r.sessions {
		if !now.Before(s.ExpiresAt) {
			delete(r.sessions, k)
			n++
		}
	}
	return n, nil
}
