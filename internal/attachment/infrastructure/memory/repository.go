// Package memory stores attachment records in memory (dev mode and tests).
package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type key struct {
	ws string
	id domain.ID
}

type Repository struct {
	mu    sync.Mutex
	items map[key]domain.Attachment
	order []key
}

func NewRepository() *Repository { return &Repository{items: map[key]domain.Attachment{}} }

func (r *Repository) Save(ctx context.Context, a *domain.Attachment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{tenant.Workspace(ctx), a.ID}
	r.items[k] = *a
	r.order = append(r.order, k)
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.ID) (*domain.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.items[key{tenant.Workspace(ctx), id}]
	if !ok {
		return nil, domain.ErrAttachmentNotFound
	}
	return &a, nil
}

func (r *Repository) ForIssue(ctx context.Context, issue string) ([]*domain.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Attachment
	for _, k := range r.order {
		if a, ok := r.items[k]; ok && k.ws == tenant.Workspace(ctx) && a.Issue == issue {
			out = append(out, &a)
		}
	}
	slices.SortStableFunc(out, func(a, b *domain.Attachment) int { return a.UploadedAt.Compare(b.UploadedAt) })
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{tenant.Workspace(ctx), id}
	if _, ok := r.items[k]; !ok {
		return domain.ErrAttachmentNotFound
	}
	delete(r.items, k)
	r.order = slices.DeleteFunc(r.order, func(o key) bool { return o == k })
	return nil
}
