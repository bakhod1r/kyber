// Package memory is the in-memory Whiteboard store (tests, dev mode).
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

type Repository struct {
	mu     sync.Mutex
	boards map[domain.WhiteboardID]*domain.Whiteboard
}

func NewRepository() *Repository {
	return &Repository{boards: map[domain.WhiteboardID]*domain.Whiteboard{}}
}

func clone(b *domain.Whiteboard) *domain.Whiteboard {
	c := *b
	c.Elements = maps.Clone(b.Elements)
	for id, e := range c.Elements {
		e.Points = slices.Clone(e.Points)
		c.Elements[id] = e
	}
	return &c
}

func (r *Repository) Create(_ context.Context, b *domain.Whiteboard) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.boards[b.ID] = clone(b)
	return nil
}

func (r *Repository) ByID(_ context.Context, id domain.WhiteboardID) (*domain.Whiteboard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.boards[id]
	if !ok {
		return nil, domain.ErrWhiteboardMissing
	}
	return clone(b), nil
}

func (r *Repository) InConversation(_ context.Context, conv domain.ConversationID) ([]*domain.Whiteboard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Whiteboard
	for _, b := range r.boards {
		if b.Conversation == conv {
			out = append(out, clone(b))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Whiteboard) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out, nil
}

// Save checks every result against the stored version (or the batch's previous result) first, so a stale
// batch changes nothing.
func (r *Repository) Save(_ context.Context, id domain.WhiteboardID, applied []domain.OpResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.boards[id]
	if !ok {
		return domain.ErrWhiteboardMissing
	}
	seen := map[domain.ElementID]int{}
	for _, res := range applied {
		e := res.Element
		prev, inBatch := seen[e.ID]
		if !inBatch {
			prev = b.Elements[e.ID].Version // 0 when absent
		}
		if e.Version != prev+1 {
			return domain.ErrStale
		}
		seen[e.ID] = e.Version
	}
	for _, res := range applied {
		if res.Element.Deleted {
			delete(b.Elements, res.Element.ID)
			continue
		}
		e := res.Element
		e.Points = slices.Clone(e.Points)
		b.Elements[e.ID] = e
	}
	return nil
}
