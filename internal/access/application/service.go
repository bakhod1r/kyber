// Package app is the Access context's use case: answer "may this user do this here?"
// through the project's permission scheme (ADR-0003). Every other context asks it.
package application

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/access/domain"
)

var (
	// ErrNotFound: the actor may not know the project (or issue) exists — report 404.
	ErrNotFound = errors.New("not found")
	// ErrArchived comes with ErrForbidden: the project is archived, so it is read-only for everyone.
	ErrArchived = errors.New("project is archived (read-only)")
	// ErrForbidden: the actor sees the project but lacks the permission — report 403.
	ErrForbidden = errors.New("forbidden")
)

// Roles answers a user's role in a project (ACL to the Project context); ok=false for non-members.
type Roles interface {
	RoleOf(ctx context.Context, project, user string) (role string, ok bool, err error)
	// Archived tells whether the project is read-only (B10).
	Archived(ctx context.Context, project string) (bool, error)
}

// Issue holds the attributes ABAC rules look at.
type Issue struct {
	Reporter, Assignee string
	Status             string
	SecurityLevel      string
}

type Service struct {
	roles   Roles
	scheme  *domain.Scheme // the default scheme
	schemes domain.SchemeRepository
}

// ErrInvalidScheme: see domain.ErrInvalidScheme.
var ErrInvalidScheme = domain.ErrInvalidScheme

// LevelConfidential is the built-in issue security level: only project admins, the reporter and
// the assignee can see such an issue — it is hidden (404) from everyone else, in every list too.
const LevelConfidential = "confidential"

// SecurityLevels lists the levels an issue can have ("" = visible to everyone who can browse).
func SecurityLevels() []domain.SecurityLevel {
	return []domain.SecurityLevel{{ID: LevelConfidential, Name: "Confidential",
		Grants: []domain.Grant{domain.Role(domain.RoleAdmin), domain.Reporter(), domain.Assignee()}}}
}

// NewService decides with the default scheme (viewer < member < admin, own-issue edits) and the
// built-in security levels; WithSchemes lets projects have schemes of their own.
func NewService(roles Roles) *Service {
	return &Service{roles: roles, scheme: withLevels(domain.DefaultScheme())}
}

// WithSchemes stores per-project permission schemes in repo.
func (s *Service) WithSchemes(repo domain.SchemeRepository) *Service {
	s.schemes = repo
	return s
}

func withLevels(scheme *domain.Scheme) *domain.Scheme {
	for _, l := range SecurityLevels() {
		_ = scheme.AddLevel(l) // static, distinct ids
	}
	return scheme
}

// schemeOf returns the project's own scheme, or the default one.
func (s *Service) schemeOf(ctx context.Context, project string) (*domain.Scheme, error) {
	if s.schemes == nil {
		return s.scheme, nil
	}
	t, ok, err := s.schemes.ProjectGrants(ctx, project)
	if err != nil || !ok {
		return s.scheme, err
	}
	scheme, err := domain.SchemeFrom(t)
	if err != nil {
		return nil, err
	}
	return withLevels(scheme), nil
}

// ProjectScheme returns the project's grants and whether they are its own (project admins only).
func (s *Service) ProjectScheme(ctx context.Context, actor, project string) (domain.GrantTable, bool, error) {
	if err := s.Can(ctx, actor, project, domain.AdministerProjects, nil); err != nil {
		return nil, false, err
	}
	scheme, _ := s.schemeOf(ctx, project) // Can has just loaded it without error
	return scheme.Table(), scheme != s.scheme, nil
}

// SetProjectScheme gives the project its own grants (project admins only). The admin role must
// keep Browse and Administer, so nobody can lock the project's admins out.
func (s *Service) SetProjectScheme(ctx context.Context, actor, project string, t domain.GrantTable) (domain.GrantTable, error) {
	if err := s.Can(ctx, actor, project, domain.AdministerProjects, nil); err != nil {
		return nil, err
	}
	scheme, err := domain.SchemeFrom(t)
	if err != nil {
		return nil, err
	}
	table := scheme.Table()
	if err := s.schemes.SaveProjectGrants(ctx, project, table); err != nil {
		return nil, err
	}
	return table, nil
}

// ResetProjectScheme returns the project to the default scheme (project admins only).
func (s *Service) ResetProjectScheme(ctx context.Context, actor, project string) error {
	if err := s.Can(ctx, actor, project, domain.AdministerProjects, nil); err != nil {
		return err
	}
	return s.schemes.DeleteProjectGrants(ctx, project)
}

// Can returns nil when actor holds perm in project (on the issue when given), ErrNotFound when
// the project or issue must stay hidden, and ErrForbidden otherwise.
func (s *Service) Can(ctx context.Context, actor, project string, perm domain.Permission, on *Issue) error {
	sub := domain.Subject{User: domain.UserID(actor)}
	if actor != "" {
		role, ok, err := s.roles.RoleOf(ctx, project, actor)
		if err != nil {
			return err
		}
		if ok {
			sub.Roles = []domain.RoleID{domain.RoleID(role)}
		}
	}
	var attrs *domain.IssueAttrs
	if on != nil {
		attrs = &domain.IssueAttrs{Reporter: domain.UserID(on.Reporter), Assignee: domain.UserID(on.Assignee),
			Status: on.Status, SecurityLevel: on.SecurityLevel}
	}
	scheme, err := s.schemeOf(ctx, project)
	if err != nil {
		return err
	}
	err = scheme.Decide(sub, perm, attrs).Err()
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return errors.Join(ErrNotFound, err)
	case errors.Is(err, domain.ErrForbidden):
		return errors.Join(ErrForbidden, err)
	}
	if perm != domain.BrowseProjects {
		archived, err := s.roles.Archived(ctx, project)
		if err != nil {
			return err
		}
		if archived {
			return errors.Join(ErrForbidden, ErrArchived)
		}
	}
	return nil
}
