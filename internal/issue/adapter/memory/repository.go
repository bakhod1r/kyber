// Package memory is an in-memory Issue repository for tests and local runs.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type Repository struct {
	mu    sync.Mutex
	byKey map[string]domain.Issue
}

func NewRepository() *Repository { return &Repository{byKey: map[string]domain.Issue{}} }

func (r *Repository) Save(_ context.Context, is *domain.Issue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey[is.Key().String()] = *is
	return nil
}

func (r *Repository) ByKey(_ context.Context, key domain.IssueKey) (*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	is, ok := r.byKey[key.String()]
	if !ok {
		return nil, domain.ErrIssueNotFound
	}
	return &is, nil
}

func (r *Repository) ListByProject(_ context.Context, project string, status *domain.StatusID) ([]*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Issue
	for _, is := range r.byKey {
		if is.Key().Project() != project || (status != nil && is.Status() != *status) {
			continue
		}
		out = append(out, &is)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key().Number() < out[j].Key().Number() })
	return out, nil
}
