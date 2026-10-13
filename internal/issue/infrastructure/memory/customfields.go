package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type fieldEntry struct {
	ws  string
	seq int
	s   domain.CustomFieldSnapshot
}

// CustomFieldRepository keeps custom field definitions in memory.
type CustomFieldRepository struct {
	mu   sync.Mutex
	seq  int
	byID map[domain.FieldID]fieldEntry
}

func NewCustomFieldRepository() *CustomFieldRepository {
	return &CustomFieldRepository{byID: map[domain.FieldID]fieldEntry{}}
}

func (r *CustomFieldRepository) SaveField(ctx context.Context, f *domain.CustomField) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := tenant.Workspace(ctx)
	e, ok := r.byID[f.ID()]
	if (ok && e.ws != ws) || ok != (f.Version() > 0) || (ok && e.s.Version != f.Version()) {
		return domain.ErrFieldConflict
	}
	for id, x := range r.byID {
		if id != f.ID() && x.ws == ws && x.s.Project == f.Project() && strings.EqualFold(x.s.Name, f.Name()) {
			return domain.ErrFieldConflict // names are unique per project
		}
	}
	if !ok {
		r.seq++
		e = fieldEntry{ws: ws, seq: r.seq}
	}
	f.MarkPersisted()
	e.s = f.Snapshot()
	r.byID[f.ID()] = e
	return nil
}

func (r *CustomFieldRepository) FieldByID(ctx context.Context, id domain.FieldID) (*domain.CustomField, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrFieldNotFound
	}
	return domain.RehydrateCustomField(e.s), nil
}

func (r *CustomFieldRepository) FieldsOf(ctx context.Context, project string) ([]*domain.CustomField, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []fieldEntry
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.s.Project == project {
			list = append(list, e)
		}
	}
	out := make([]*domain.CustomField, len(list))
	for _, e := range list {
		n := 0
		for _, x := range list {
			if x.seq < e.seq {
				n++
			}
		}
		out[n] = domain.RehydrateCustomField(e.s)
	}
	return out, nil
}

func (r *CustomFieldRepository) DeleteField(ctx context.Context, id domain.FieldID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrFieldNotFound
	}
	delete(r.byID, id)
	return nil
}
