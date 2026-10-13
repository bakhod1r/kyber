// Package memory is the in-memory Call store (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/call/domain"
)

type Repository struct {
	mu    sync.Mutex
	calls map[domain.CallID]domain.Call
}

func NewRepository() *Repository { return &Repository{calls: map[domain.CallID]domain.Call{}} }

func clone(c domain.Call) *domain.Call {
	c.Participants = slices.Clone(c.Participants)
	if c.Presentation != nil {
		p := *c.Presentation
		c.Presentation = &p
	}
	return &c
}

func (r *Repository) Create(_ context.Context, c *domain.Call) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.calls {
		if o.Conversation == c.Conversation && o.Live() {
			return domain.ErrCallLive
		}
	}
	r.calls[c.ID] = *clone(*c)
	return nil
}

func (r *Repository) Save(_ context.Context, c *domain.Call) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.calls[c.ID]
	if !ok {
		return domain.ErrCallNotFound
	}
	if old.Version != c.Version {
		return domain.ErrConflict
	}
	c.Version++
	r.calls[c.ID] = *clone(*c)
	return nil
}

func (r *Repository) ByID(_ context.Context, id domain.CallID) (*domain.Call, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.calls[id]
	if !ok {
		return nil, domain.ErrCallNotFound
	}
	return clone(c), nil
}

func (r *Repository) LiveIn(_ context.Context, conv domain.ConversationID) (*domain.Call, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.Conversation == conv && c.Live() {
			return clone(c), nil
		}
	}
	return nil, domain.ErrCallNotFound
}
