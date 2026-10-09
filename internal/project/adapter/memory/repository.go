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
	byKey map[string]domain.Project
}

func NewRepository() *Repository { return &Repository{byKey: map[string]domain.Project{}} }

func (r *Repository) Create(_ context.Context, p *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byKey[p.Key()]; ok {
		return domain.ErrKeyTaken
	}
	r.byKey[p.Key()] = *p
	return nil
}

func (r *Repository) Update(_ context.Context, key string, fn func(*domain.Project) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byKey[key]
	if !ok {
		return domain.ErrProjectNotFound
	}
	if err := fn(&p); err != nil {
		return err
	}
	r.byKey[key] = p
	return nil
}

func (r *Repository) ByKey(_ context.Context, key string) (*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byKey[key]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return &p, nil
}

func (r *Repository) List(_ context.Context) ([]*domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Project, 0, len(r.byKey))
	for _, p := range r.byKey {
		out = append(out, &p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out, nil
}
