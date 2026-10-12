package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type boardEntry struct {
	ws, project, name string
	kind              domain.BoardKind
	columns           []domain.Column
	version           int
}

// BoardRepository keeps boards in memory.
type BoardRepository struct {
	mu   sync.Mutex
	byID map[domain.BoardID]boardEntry
}

func NewBoardRepository() *BoardRepository {
	return &BoardRepository{byID: map[domain.BoardID]boardEntry{}}
}

func (r *BoardRepository) SaveBoard(ctx context.Context, b *domain.Board) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[b.ID()]
	if (ok && e.ws != tenant.Workspace(ctx)) || ok != (b.Version() > 0) || (ok && e.version != b.Version()) {
		return domain.ErrBoardConflict
	}
	b.MarkPersisted()
	r.byID[b.ID()] = boardEntry{ws: tenant.Workspace(ctx), project: b.Project(), name: b.Name(), kind: b.Kind(),
		columns: b.Columns(), version: b.Version()}
	return nil
}

func (r *BoardRepository) BoardByID(ctx context.Context, id domain.BoardID) (*domain.Board, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrBoardNotFound
	}
	return domain.RehydrateBoard(id, e.project, e.name, e.kind, e.columns, e.version), nil
}

func (r *BoardRepository) BoardsByProject(ctx context.Context, project string) ([]*domain.Board, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*domain.Board{}
	for id, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.project == project {
			out = append(out, domain.RehydrateBoard(id, e.project, e.name, e.kind, e.columns, e.version))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name() != out[j].Name() {
			return out[i].Name() < out[j].Name()
		}
		return out[i].ID() < out[j].ID()
	})
	return out, nil
}

func (r *BoardRepository) DeleteBoard(ctx context.Context, id domain.BoardID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.byID[id]; !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrBoardNotFound
	}
	delete(r.byID, id)
	return nil
}
