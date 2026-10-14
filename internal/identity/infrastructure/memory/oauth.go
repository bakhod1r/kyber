package memory

import (
	"context"
	"slices"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func (r *Repository) CreateClient(_ context.Context, c *domain.OAuthClient) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clients == nil {
		r.clients = map[domain.ClientID]domain.OAuthClient{}
	}
	cp := *c
	cp.RedirectURIs = slices.Clone(c.RedirectURIs)
	r.clients[c.ID] = cp
	return nil
}

func (r *Repository) Client(_ context.Context, id domain.ClientID) (*domain.OAuthClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clients[id]
	if !ok {
		return nil, domain.ErrOAuthClientNotFound
	}
	c.RedirectURIs = slices.Clone(c.RedirectURIs)
	return &c, nil
}

func (r *Repository) SaveCode(_ context.Context, c *domain.AuthCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.codes == nil {
		r.codes = map[string]domain.AuthCode{}
	}
	r.codes[string(c.Hash)] = *c
	return nil
}

func (r *Repository) TakeCode(_ context.Context, hash []byte) (*domain.AuthCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.codes[string(hash)]
	if !ok {
		return nil, domain.ErrAuthCodeNotFound
	}
	delete(r.codes, string(hash))
	return &c, nil
}

var _ domain.OAuthStore = (*Repository)(nil)
