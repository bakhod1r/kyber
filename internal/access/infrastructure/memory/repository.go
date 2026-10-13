// Package memory is the in-memory permission-scheme store (tests, dev mode).
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct {
	mu     sync.Mutex
	tables map[string]domain.GrantTable // workspace/project
}

func NewRepository() *Repository { return &Repository{tables: map[string]domain.GrantTable{}} }

func key(ctx context.Context, project string) string { return tenant.Workspace(ctx) + "/" + project }

func clone(t domain.GrantTable) domain.GrantTable {
	out := domain.GrantTable{}
	for p, gs := range t {
		out[p] = slices.Clone(gs)
	}
	return out
}

func (r *Repository) ProjectGrants(ctx context.Context, project string) (domain.GrantTable, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tables[key(ctx, project)]
	if !ok {
		return nil, false, nil
	}
	return clone(t), true, nil
}

func (r *Repository) SaveProjectGrants(ctx context.Context, project string, t domain.GrantTable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tables[key(ctx, project)] = clone(t)
	return nil
}

func (r *Repository) DeleteProjectGrants(ctx context.Context, project string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	maps.DeleteFunc(r.tables, func(k string, _ domain.GrantTable) bool { return k == key(ctx, project) })
	return nil
}
