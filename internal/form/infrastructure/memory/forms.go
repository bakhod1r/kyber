// Package memory keeps forms in memory (tests, dev mode).
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/form/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type entry struct {
	ws string
	f  domain.Form
}

type Repository struct {
	mu   sync.Mutex
	byID map[domain.FormID]entry
}

func NewRepository() *Repository { return &Repository{byID: map[domain.FormID]entry{}} }

// clone copies the form so callers never share its questions.
func clone(f domain.Form) *domain.Form {
	f.Questions = slices.Clone(f.Questions)
	return &f
}

func (r *Repository) Save(ctx context.Context, f *domain.Form) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[f.ID]
	if (ok && e.ws != tenant.Workspace(ctx)) || ok != (f.Version > 0) || (ok && e.f.Version != f.Version) {
		return domain.ErrFormConflict
	}
	f.Version++
	r.byID[f.ID] = entry{ws: tenant.Workspace(ctx), f: *clone(*f)}
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.FormID) (*domain.Form, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrFormNotFound
	}
	return clone(e.f), nil
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Form, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Form
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.f.Project == project {
			out = append(out, clone(e.f))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Form) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, id domain.FormID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrFormNotFound
	}
	delete(r.byID, id)
	return nil
}
