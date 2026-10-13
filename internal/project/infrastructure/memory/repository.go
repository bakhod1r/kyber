// Package memory is an in-memory Project repository for tests and local runs.
package memory

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Repository struct {
	mu    sync.Mutex
	byKey map[string]*domain.Project
}

func NewRepository() *Repository { return &Repository{byKey: map[string]*domain.Project{}} }

// clone deep-copies a project so callers never share the stored membership map.
func clone(p *domain.Project) *domain.Project {
	return domain.Rehydrate(p.ID(), p.Workspace(), p.Key(), p.Name(), p.IssueSeq(), p.Members())
}

// slot is the map key: project keys are unique per workspace (ADR-0004 phase 2).
func slot(ws, key string) string { return ws + "/" + key }

func (r *Repository) Create(_ context.Context, p *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := slot(string(p.Workspace()), p.Key())
	if _, ok := r.byKey[k]; ok {
		return domain.ErrKeyTaken
	}
	r.byKey[k] = clone(p)
	return nil
}

func (r *Repository) Update(ctx context.Context, key string, fn func(*domain.Project) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key = slot(tenant.Workspace(ctx), key)
	p, ok := r.byKey[key]
	if !ok {
		return domain.ErrProjectNotFound
	}
	cp := clone(p)
	if err := fn(cp); err != nil {
		return err
	}
	r.byKey[key] = cp
	return nil
}

func (r *Repository) ByKey(ctx context.Context, key string) (*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byKey[slot(tenant.Workspace(ctx), key)]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return clone(p), nil
}

func (r *Repository) ListForUser(ctx context.Context, u domain.UserID) ([]*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := domain.WorkspaceID(tenant.Workspace(ctx))
	var out []*domain.Project
	for _, p := range r.byKey {
		if _, ok := p.RoleOf(u); ok && p.Workspace() == ws {
			out = append(out, clone(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, nil
}
