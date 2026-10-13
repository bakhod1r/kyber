// Package memory is the in-memory workspace store (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

type row struct {
	slug, name string
	members    []domain.Member
}

type Repository struct {
	mu   sync.Mutex
	rows map[domain.WorkspaceID]row
}

func NewRepository() *Repository { return &Repository{rows: map[domain.WorkspaceID]row{}} }

func (r *Repository) Create(_ context.Context, w *domain.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.rows {
		if x.slug == w.Slug() {
			return domain.ErrSlugTaken
		}
	}
	r.rows[w.ID()] = row{w.Slug(), w.Name(), w.Members()}
	return nil
}

func (r *Repository) load(id domain.WorkspaceID, x row) *domain.Workspace {
	return domain.Rehydrate(id, x.slug, x.name, x.members)
}

func (r *Repository) BySlug(_ context.Context, slug string) (*domain.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, x := range r.rows {
		if x.slug == slug {
			return r.load(id, x), nil
		}
	}
	return nil, domain.ErrWorkspaceNotFound
}

func (r *Repository) ByID(_ context.Context, id domain.WorkspaceID) (*domain.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if x, ok := r.rows[id]; ok {
		return r.load(id, x), nil
	}
	return nil, domain.ErrWorkspaceNotFound
}

func (r *Repository) ForUser(_ context.Context, u domain.UserID) ([]*domain.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Workspace
	for id, x := range r.rows {
		if w := r.load(id, x); w.IsMember(u) {
			out = append(out, w)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Workspace) int { return strings.Compare(a.Slug(), b.Slug()) })
	return out, nil
}

func (r *Repository) SaveMembers(_ context.Context, w *domain.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := r.rows[w.ID()]
	x.members = w.Members()
	r.rows[w.ID()] = x
	return nil
}

func (r *Repository) AddMember(_ context.Context, id domain.WorkspaceID, u domain.UserID, role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x := r.rows[id]
	for _, m := range x.members {
		if m.UserID == u {
			return nil
		}
	}
	x.members = append(x.members, domain.Member{UserID: u, Role: role})
	r.rows[id] = x
	return nil
}
