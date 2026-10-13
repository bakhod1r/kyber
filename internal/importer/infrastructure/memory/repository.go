// Package memory is the in-memory import mapping store (tests, dev mode).
package memory

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"maps"
	"sync"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

type Repository struct {
	mu   sync.Mutex
	byPK map[string]map[string]string
}

func NewRepository() *Repository { return &Repository{byPK: map[string]map[string]string{}} }

func (r *Repository) Add(ctx context.Context, m domain.Mapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := tenant.Workspace(ctx) + "/" + m.Project // per workspace (ADR-0004 phase 2)
	if _, ok := r.byPK[p][m.ExternalKey]; ok {
		return domain.ErrAlreadyImported
	}
	if r.byPK[p] == nil {
		r.byPK[p] = map[string]string{}
	}
	r.byPK[p][m.ExternalKey] = m.IssueKey
	return nil
}

func (r *Repository) ForProject(ctx context.Context, project string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := maps.Clone(r.byPK[tenant.Workspace(ctx)+"/"+project])
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}
