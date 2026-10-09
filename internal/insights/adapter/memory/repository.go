// Package memory is an in-memory activity log for tests and dev mode.
package memory

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/insights/domain"
)

type key struct {
	event int64
	issue string
	kind  domain.Kind
}

type row struct {
	domain.Entry
	ws string // workspace (ADR-0004 phase 2)
}

type Repository struct {
	mu   sync.Mutex
	all  []row
	seen map[key]bool
}

func NewRepository() *Repository { return &Repository{seen: map[key]bool{}} }

func (r *Repository) Record(ctx context.Context, entries ...domain.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range entries {
		k := key{e.Event, e.Issue, e.Kind}
		if !r.seen[k] {
			r.seen[k] = true
			r.all = append(r.all, row{e, tenant.Workspace(ctx)})
		}
	}
	return nil
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]domain.Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Entry
	for _, e := range r.all {
		if e.ws == tenant.Workspace(ctx) && e.Project == project {
			out = append(out, e.Entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].Event < out[j].Event
	})
	return out, nil
}
