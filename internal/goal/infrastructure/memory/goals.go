// Package memory keeps goals in memory (tests, dev mode).
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/goal/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type entry struct {
	ws string
	g  domain.Goal
}

type Repository struct {
	mu   sync.Mutex
	byID map[domain.GoalID]entry
}

func NewRepository() *Repository { return &Repository{byID: map[domain.GoalID]entry{}} }

func (r *Repository) Save(ctx context.Context, g *domain.Goal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[g.ID]
	if (ok && e.ws != tenant.Workspace(ctx)) || ok != (g.Version > 0) || (ok && e.g.Version != g.Version) {
		return domain.ErrGoalConflict
	}
	g.Version++
	r.byID[g.ID] = entry{ws: tenant.Workspace(ctx), g: *g}
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.GoalID) (*domain.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrGoalNotFound
	}
	g := e.g
	return &g, nil
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Goal
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.g.Project == project {
			g := e.g
			out = append(out, &g)
		}
	}
	// No target date sorts last, as NULLS LAST does in PostgreSQL.
	target := func(g *domain.Goal) string { return cmp.Or(g.Target, "9999-99-99") }
	slices.SortFunc(out, func(a, b *domain.Goal) int {
		return cmp.Or(cmp.Compare(target(a), target(b)), cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, id domain.GoalID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrGoalNotFound
	}
	delete(r.byID, id)
	return nil
}
