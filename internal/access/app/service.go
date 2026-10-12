// Package app is the Access context's use case: answer "may this user do this here?"
// through the project's permission scheme (ADR-0003). Every other context asks it.
package app

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/access/domain"
)

var (
	// ErrNotFound: the actor may not know the project (or issue) exists — report 404.
	ErrNotFound = errors.New("not found")
	// ErrForbidden: the actor sees the project but lacks the permission — report 403.
	ErrForbidden = errors.New("forbidden")
)

// Roles answers a user's role in a project (ACL to the Project context); ok=false for non-members.
type Roles interface {
	RoleOf(ctx context.Context, project, user string) (role string, ok bool, err error)
}

// Issue holds the attributes ABAC rules look at.
type Issue struct {
	Reporter, Assignee string
	Status             string
	SecurityLevel      string
}

type Service struct {
	roles  Roles
	scheme *domain.Scheme
}

// NewService decides with the default scheme (viewer < member < admin, own-issue edits);
// per-project schemes and security levels come in phase 2.
func NewService(roles Roles) *Service { return &Service{roles: roles, scheme: domain.DefaultScheme()} }

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
	err := s.scheme.Decide(sub, perm, attrs).Err()
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return errors.Join(ErrNotFound, err)
	case errors.Is(err, domain.ErrForbidden):
		return errors.Join(ErrForbidden, err)
	}
	return err
}
