// Package app holds the Workspace context use cases.
package app

import (
	"context"

	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

type Service struct {
	repo  domain.Repository
	newID func() string
}

func NewService(repo domain.Repository, newID func() string) *Service {
	return &Service{repo: repo, newID: newID}
}

// Create makes a workspace whose creator is its owner.
func (s *Service) Create(ctx context.Context, actor, slug, name string) (*domain.Workspace, error) {
	w, err := domain.NewWorkspace(domain.WorkspaceID(s.newID()), slug, name, domain.UserID(actor))
	if err != nil {
		return nil, err
	}
	return w, s.repo.Create(ctx, w)
}

// Mine lists the actor's workspaces.
func (s *Service) Mine(ctx context.Context, actor string) ([]*domain.Workspace, error) {
	return s.repo.ForUser(ctx, domain.UserID(actor))
}

// Resolve finds a workspace by its subdomain slug.
func (s *Service) Resolve(ctx context.Context, slug string) (*domain.Workspace, error) {
	return s.repo.BySlug(ctx, slug)
}

// IsMember answers whether user may use the workspace; the default workspace is open.
func (s *Service) IsMember(ctx context.Context, workspace, user string) (bool, error) {
	if domain.WorkspaceID(workspace) == domain.DefaultID {
		return true, nil
	}
	w, err := s.repo.ByID(ctx, domain.WorkspaceID(workspace))
	if err != nil {
		return false, err
	}
	return w.IsMember(domain.UserID(user)), nil
}

// Join makes user a member (used when a project admin adds someone to a project).
// Existing members keep their role.
func (s *Service) Join(ctx context.Context, workspace, user string) error {
	if domain.WorkspaceID(workspace) == domain.DefaultID {
		return nil
	}
	if _, err := s.repo.ByID(ctx, domain.WorkspaceID(workspace)); err != nil {
		return err
	}
	return s.repo.AddMember(ctx, domain.WorkspaceID(workspace), domain.UserID(user), domain.RoleMember)
}
