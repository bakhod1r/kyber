// Package memory keeps saved filters in memory (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/search/domain"
)

type entry struct {
	ws string
	f  domain.Filter
}

type FilterRepository struct {
	mu   sync.Mutex
	byID map[domain.FilterID]entry
}

func NewFilterRepository() *FilterRepository {
	return &FilterRepository{byID: map[domain.FilterID]entry{}}
}

func (r *FilterRepository) SaveFilter(ctx context.Context, f *domain.Filter) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[f.ID]
	if (ok && e.ws != tenant.Workspace(ctx)) || ok != (f.Version > 0) || (ok && e.f.Version != f.Version) {
		return domain.ErrFilterConflict
	}
	f.Version++
	r.byID[f.ID] = entry{ws: tenant.Workspace(ctx), f: *f}
	return nil
}

func (r *FilterRepository) FilterByID(ctx context.Context, id domain.FilterID) (*domain.Filter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrFilterNotFound
	}
	f := e.f
	return &f, nil
}

func (r *FilterRepository) FiltersFor(ctx context.Context, user string) ([]*domain.Filter, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Filter
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && (e.f.Owner == user || e.f.Shared) {
			f := e.f
			out = append(out, &f)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Filter) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *FilterRepository) DeleteFilter(ctx context.Context, id domain.FilterID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrFilterNotFound
	}
	delete(r.byID, id)
	return nil
}
