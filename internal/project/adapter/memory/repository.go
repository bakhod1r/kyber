// Package memory is an in-memory Project repository for tests and local runs.
package memory

import (
	"context"
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
	return domain.Rehydrate(p.ID(), p.Key(), p.Name(), p.IssueSeq(), p.Members())
}

func (r *Repository) Create(_ context.Context, p *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[p.Key()]; ok {
		return domain.ErrKeyTaken
	}
	r.byKey[p.Key()] = clone(p)
	return nil
}

func (r *Repository) Update(_ context.Context, key string, fn func(*domain.Project) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *Repository) ByKey(_ context.Context, key string) (*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return clone(p), nil
}

func (r *Repository) ListForUser(_ context.Context, u domain.UserID) ([]*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Project
	for _, p := range r.byKey {
		if _, ok := p.RoleOf(u); ok {
			out = append(out, clone(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, nil
}
