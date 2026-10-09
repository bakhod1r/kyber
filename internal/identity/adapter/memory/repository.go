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
	external map[domain.Provider]map[string]domain.UserID
	otps     map[string]domain.OTPChallenge
}

func NewRepository() *Repository {
	return &Repository{users: map[domain.UserID]domain.User{}, sessions: map[string]domain.Session{},
		external: map[domain.Provider]map[string]domain.UserID{}, otps: map[string]domain.OTPChallenge{}}
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

func (r *Repository) LinkedUser(_ context.Context, p domain.Provider, subject string) (domain.UserID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id, ok := r.external[p][subject]; ok {
		return id, nil
	}
	return "", domain.ErrNotLinked
}

func (r *Repository) Link(_ context.Context, id domain.ExternalIdentity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.external[id.Provider] == nil {
		r.external[id.Provider] = map[string]domain.UserID{}
	}
	if _, linked := r.external[id.Provider][id.Subject]; !linked {
		r.external[id.Provider][id.Subject] = id.UserID
	}
	return nil
}

func (r *Repository) CreateOTP(_ context.Context, c *domain.OTPChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.otps[c.ID] = *c
	return nil
}

func (r *Repository) OTPByID(_ context.Context, id string) (*domain.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.otps[id]; ok {
		return &c, nil
	}
	return nil, domain.ErrOTPNotFound
}

func (r *Repository) OTPByNonce(_ context.Context, nonce string) (*domain.OTPChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.otps {
		if c.Nonce == nonce {
			return &c, nil
		}
	}
	return nil, domain.ErrOTPNotFound
}

func (r *Repository) UpdateOTP(_ context.Context, id string, fn func(*domain.OTPChallenge) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.otps[id]
	if !ok {
		return domain.ErrOTPNotFound
	}
	err := fn(&c)
	r.otps[id] = c
	return err
}
