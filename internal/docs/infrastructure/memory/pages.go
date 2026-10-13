// Package memory keeps pages in memory (tests, dev mode).
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type entry struct {
	ws   string
	p    domain.Page
	revs []domain.Revision // oldest first
}

type Repository struct {
	mu   sync.Mutex
	byID map[domain.PageID]*entry
}

func NewRepository() *Repository { return &Repository{byID: map[domain.PageID]*entry{}} }

func (r *Repository) find(ctx context.Context, id domain.PageID) (*entry, bool) {
	e, ok := r.byID[id]
	return e, ok && e.ws == tenant.Workspace(ctx)
}

func (r *Repository) Save(ctx context.Context, p *domain.Page, rev domain.Revision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.byID[p.ID]
	if (exists && e.ws != tenant.Workspace(ctx)) || exists != (p.Version > 0) || (exists && e.p.Version != p.Version) {
		return domain.ErrPageConflict
	}
	if !exists {
		e = &entry{ws: tenant.Workspace(ctx)}
		r.byID[p.ID] = e
	}
	p.Version = rev.Number
	e.p = *p
	e.revs = append(e.revs, rev)
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.PageID) (*domain.Page, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.find(ctx, id)
	if !ok {
		return nil, domain.ErrPageNotFound
	}
	p := e.p
	return &p, nil
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Page, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Page
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.p.Project == project {
			p := e.p
			out = append(out, &p)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Page) int { return cmp.Or(cmp.Compare(a.Title, b.Title), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}

func (r *Repository) History(ctx context.Context, id domain.PageID) ([]domain.Revision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.find(ctx, id)
	if !ok {
		return nil, nil
	}
	out := slices.Clone(e.revs)
	slices.Reverse(out)
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, id domain.PageID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.find(ctx, id); !ok {
		return domain.ErrPageNotFound
	}
	delete(r.byID, id)
	return nil
}
