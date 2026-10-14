package memory

import (
	"context"
	"slices"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func (r *Repository) CreateToken(_ context.Context, t *domain.PersonalToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens == nil {
		r.tokens = map[domain.TokenID]domain.PersonalToken{}
	}
	r.tokens[t.ID] = *t
	return nil
}

func (r *Repository) TokenByHash(_ context.Context, hash []byte) (*domain.PersonalToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tokens {
		if string(t.Hash) == string(hash) {
			return &t, nil
		}
	}
	return nil, domain.ErrTokenNotFound
}

func (r *Repository) TokensOf(_ context.Context, user domain.UserID) ([]*domain.PersonalToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.PersonalToken
	for _, t := range r.tokens {
		if t.UserID == user {
			out = append(out, &t)
		}
	}
	slices.SortFunc(out, func(a, b *domain.PersonalToken) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out, nil
}

func (r *Repository) DeleteToken(_ context.Context, user domain.UserID, id domain.TokenID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[id]; !ok || t.UserID != user {
		return domain.ErrTokenNotFound
	}
	delete(r.tokens, id)
	return nil
}

func (r *Repository) TouchToken(_ context.Context, id domain.TokenID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[id]; ok {
		t.LastUsedAt = &at
		r.tokens[id] = t
	}
	return nil
}
