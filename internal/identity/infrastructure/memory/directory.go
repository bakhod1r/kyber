package memory

import (
	"context"
	"sort"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func (r *Repository) SaveUser(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[u.ID()]; !ok {
		return domain.ErrUserNotFound
	}
	r.users[u.ID()] = *u
	return nil
}

func (r *Repository) ListUsers(_ context.Context, email domain.Email, offset, limit int) ([]*domain.User, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []*domain.User
	for _, u := range r.users {
		if email == "" || u.Email() == email {
			all = append(all, &u)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Email() < all[j].Email() })
	total := len(all)
	if offset > total {
		offset = total
	}
	return all[offset:min(total, offset+limit)], total, nil
}

func (r *Repository) DeleteSessionsOf(_ context.Context, id domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for h, s := range r.sessions {
		if s.UserID == id {
			delete(r.sessions, h)
		}
	}
	return nil
}

var _ domain.Directory = (*Repository)(nil)
