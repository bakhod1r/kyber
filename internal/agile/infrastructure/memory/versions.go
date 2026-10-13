package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type versionEntry struct {
	ws string
	s  domain.VersionSnapshot
}

// VersionRepository keeps versions in memory.
type VersionRepository struct {
	mu   sync.Mutex
	byID map[domain.VersionID]versionEntry
}

func NewVersionRepository() *VersionRepository {
	return &VersionRepository{byID: map[domain.VersionID]versionEntry{}}
}

func (r *VersionRepository) SaveVersion(ctx context.Context, v *domain.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws := tenant.Workspace(ctx)
	e, ok := r.byID[v.ID()]
	if (ok && e.ws != ws) || ok != (v.Version() > 0) || (ok && e.s.Version != v.Version()) {
		return domain.ErrVersionConflict
	}
	for id, x := range r.byID {
		if id != v.ID() && x.ws == ws && x.s.Project == v.Project() && x.s.Name == v.Name() {
			return domain.ErrVersionTaken
		}
	}
	v.MarkPersisted()
	r.byID[v.ID()] = versionEntry{ws: ws, s: v.Snapshot()}
	return nil
}

func (r *VersionRepository) VersionByID(ctx context.Context, id domain.VersionID) (*domain.Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrVersionNotFound
	}
	return domain.RehydrateVersion(e.s), nil
}

func (r *VersionRepository) VersionsOf(ctx context.Context, project string) ([]*domain.Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []domain.VersionSnapshot
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.s.Project == project {
			list = append(list, e.s)
		}
	}
	slices.SortFunc(list, compareVersions)
	out := make([]*domain.Version, 0, len(list))
	for _, s := range list {
		out = append(out, domain.RehydrateVersion(s))
	}
	return out, nil
}

// compareVersions orders dated versions first by date, then the rest, by name within each.
func compareVersions(a, b domain.VersionSnapshot) int {
	if az, bz := a.ReleaseDate.IsZero(), b.ReleaseDate.IsZero(); az != bz {
		if az {
			return 1
		}
		return -1
	}
	if c := a.ReleaseDate.Compare(b.ReleaseDate); c != 0 {
		return c
	}
	return cmp.Compare(a.Name, b.Name)
}

func (r *VersionRepository) DeleteVersion(ctx context.Context, id domain.VersionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return domain.ErrVersionNotFound
	}
	delete(r.byID, id)
	return nil
}
