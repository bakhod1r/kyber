// Package memory is an in-memory audit log for tests and dev mode.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type row struct {
	ws  string
	seq int
	e   domain.Entry
}

type Repository struct {
	mu   sync.Mutex
	rows []row
}

func NewRepository() *Repository { return &Repository{} }

func (r *Repository) Append(ctx context.Context, e domain.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows = append(r.rows, row{ws: tenant.Workspace(ctx), seq: len(r.rows), e: e})
	return nil
}

func (r *Repository) ForProject(ctx context.Context, project string, limit int) ([]domain.Entry, error) {
	r.mu.Lock()
	var rows []row
	for _, x := range r.rows {
		if x.ws == tenant.Workspace(ctx) && x.e.Project == project {
			rows = append(rows, x)
		}
	}
	r.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].e.At.Equal(rows[j].e.At) {
			return rows[i].e.At.After(rows[j].e.At)
		}
		return rows[i].seq > rows[j].seq
	})
	out := []domain.Entry{}
	for i := 0; i < len(rows) && i < limit; i++ {
		out = append(out, rows[i].e)
	}
	return out, nil
}
