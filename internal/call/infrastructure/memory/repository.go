// Package memory is the in-memory Call store (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type Repository struct {
	mu     sync.Mutex
	calls  map[domain.CallID]domain.Call
	outbox []domain.Event
	outAt  []outbox.Meta
}

// Outbox returns the published events written so far; OutboxMeta their envelope.
func (r *Repository) Outbox() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Event(nil), r.outbox...)
}

func (r *Repository) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]outbox.Meta(nil), r.outAt...)
}

func (r *Repository) record(ctx context.Context, c *domain.Call) {
	for _, e := range c.PullEvents() {
		r.outbox, r.outAt = append(r.outbox, e), append(r.outAt, outbox.MetaOf(ctx, time.Now()))
	}
}

func NewRepository() *Repository { return &Repository{calls: map[domain.CallID]domain.Call{}} }

func clone(c domain.Call) *domain.Call {
	c.Participants = slices.Clone(c.Participants)
	if c.Presentation != nil {
		p := *c.Presentation
		c.Presentation = &p
	}
	if c.Recording != nil {
		r := *c.Recording
		c.Recording = &r
	}
	return &c
}

func (r *Repository) Create(ctx context.Context, c *domain.Call) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.calls {
		if o.Conversation == c.Conversation && o.Live() {
			return domain.ErrCallLive
		}
	}
	r.record(ctx, c)
	r.calls[c.ID] = *clone(*c)
	return nil
}

func (r *Repository) Save(ctx context.Context, c *domain.Call) error {
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
	r.record(ctx, c)
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

func (r *Repository) CountLive(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c.Live() {
			n++
		}
	}
	return n, nil
}
