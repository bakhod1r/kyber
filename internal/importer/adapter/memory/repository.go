// Package memory is the in-memory import mapping store (tests, dev mode).
package memory

import (
	"context"
	"maps"
	"sync"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

type Repository struct {
	mu   sync.Mutex
	byPK map[string]map[string]string
}

func NewRepository() *Repository { return &Repository{byPK: map[string]map[string]string{}} }

func (r *Repository) Add(_ context.Context, m domain.Mapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byPK[m.Project][m.ExternalKey]; ok {
		return domain.ErrAlreadyImported
	}
	if r.byPK[m.Project] == nil {
		r.byPK[m.Project] = map[string]string{}
	}
	r.byPK[m.Project][m.ExternalKey] = m.IssueKey
	return nil
}

func (r *Repository) ForProject(_ context.Context, project string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := maps.Clone(r.byPK[project])
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}
