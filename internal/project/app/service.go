// Package app holds the Project context use cases.
package app

import (
	"context"
	"slices"

	"errors"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

var ErrUnknownUser = errors.New("no user with that email")

// UserInfo is the Identity context's view of a user, translated by the directory adapter.
type UserInfo struct {
	ID    domain.UserID
	Email string
	Name  string
}

// UserDirectory is the anti-corruption port to the Identity context.
type UserDirectory interface {
	ByEmail(ctx context.Context, email string) (UserInfo, error) // ErrUnknownUser
	ByID(ctx context.Context, id domain.UserID) (UserInfo, error)
}

type Service struct {
	repo       domain.Repository
	users      UserDirectory
	newID      func() string
	workspaces Workspaces // nil = no workspace membership sync
}

func NewService(repo domain.Repository, users UserDirectory, newID func() string) *Service {
	return &Service{repo: repo, users: users, newID: newID}
}

// Workspaces is the anti-corruption port to the Workspace context.
type Workspaces interface {
	Join(ctx context.Context, workspace, user string) error
}

// WithWorkspaces makes project members members of the project's workspace too.
func (s *Service) WithWorkspaces(w Workspaces) *Service { s.workspaces = w; return s }

func (s *Service) Create(ctx context.Context, actor domain.UserID, key, name string) (*domain.Project, error) {
	ws := domain.DefaultWorkspace
	if t, ok := tenant.From(ctx); ok {
		ws = domain.WorkspaceID(t)
	}
	p, err := domain.NewProject(domain.ProjectID(s.newID()), ws, key, name, actor)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// byKey loads a project visible from the request's workspace; a project of another
// workspace does not exist for this request (ADR-0004).
func (s *Service) byKey(ctx context.Context, key string) (*domain.Project, error) {
	p, err := s.repo.ByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if t, ok := tenant.From(ctx); ok && domain.WorkspaceID(t) != p.Workspace() {
		return nil, domain.ErrProjectNotFound
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, actor domain.UserID, key string) (*domain.Project, error) {
	p, err := s.byKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := p.Authorize(actor, domain.PermRead); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, actor domain.UserID) ([]*domain.Project, error) {
	all, err := s.repo.ListForUser(ctx, actor)
	if err != nil {
		return nil, err
	}
	t, ok := tenant.From(ctx)
	if !ok {
		return all, nil
	}
	return slices.DeleteFunc(all, func(p *domain.Project) bool { return p.Workspace() != domain.WorkspaceID(t) }), nil
}

// Authorize checks the actor's permission on a project (used by other contexts via ACL).
func (s *Service) Authorize(ctx context.Context, actor domain.UserID, key string, need domain.Permission) error {
	p, err := s.byKey(ctx, key)
	if err != nil {
		return err
	}
	return p.Authorize(actor, need)
}

// SetMember adds a user (by email) or changes their role; admin only.
func (s *Service) SetMember(ctx context.Context, actor domain.UserID, key, email, rawRole string) error {
	role, err := domain.ParseRole(rawRole)
	if err != nil {
		return err
	}
	if err := s.Authorize(ctx, actor, key, domain.PermAdmin); err != nil {
		return err
	}
	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	var ws domain.WorkspaceID
	err = s.repo.Update(ctx, key, func(p *domain.Project) error {
		if err := p.Authorize(actor, domain.PermAdmin); err != nil { // re-check under lock
			return err
		}
		ws = p.Workspace()
		return p.SetMember(u.ID, role)
	})
	if err != nil || s.workspaces == nil {
		return err
	}
	return s.workspaces.Join(ctx, string(ws), string(u.ID))
}

type MemberInfo struct {
	UserInfo
	Role domain.Role
}

func (s *Service) Members(ctx context.Context, actor domain.UserID, key string) ([]MemberInfo, error) {
	p, err := s.Get(ctx, actor, key)
	if err != nil {
		return nil, err
	}
	out := make([]MemberInfo, 0, len(p.Members()))
	for _, m := range p.Members() {
		u, err := s.users.ByID(ctx, m.UserID)
		if err != nil {
			return nil, err
		}
		out = append(out, MemberInfo{UserInfo: u, Role: m.Role})
	}
	return out, nil
}

// WorkspaceOf returns a project's workspace for read models in other contexts (no
// authorization: callers scope their own data with it).
func (s *Service) WorkspaceOf(ctx context.Context, key string) (domain.WorkspaceID, error) {
	p, err := s.repo.ByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return p.Workspace(), nil
}

// NextIssueNumber atomically advances the project's issue sequence.
func (s *Service) NextIssueNumber(ctx context.Context, key string) (int, error) {
	var n int
	err := s.repo.Update(ctx, key, func(p *domain.Project) error {
		n = p.NextIssueNumber()
		return nil
	})
	return n, err
}
