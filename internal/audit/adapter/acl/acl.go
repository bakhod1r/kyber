// Package acl is the Audit context's anti-corruption layer to Project and Identity.
package acl

import (
	"context"
	"errors"

	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/audit/app"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Projects interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Admins struct{ projects Projects }

func NewAdmins(p Projects) Admins { return Admins{projects: p} }

func (a Admins) Administer(ctx context.Context, actor, project string) error {
	err := a.projects.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermAdmin)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return app.ErrNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return app.ErrForbidden
	}
	return err
}

type Names struct{ users identitydomain.Users }

func NewNames(u identitydomain.Users) Names { return Names{users: u} }

func (n Names) DisplayName(ctx context.Context, id string) (string, error) {
	u, err := n.users.ByID(ctx, identitydomain.UserID(id))
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return "Deleted user", nil
	}
	if err != nil {
		return "", err
	}
	return u.Name(), nil
}

// Sprints and boards are looked up without authorization: only their project key is used,
// to file the audit entry; reading the log itself requires project admin.
type SprintSource interface {
	Peek(ctx context.Context, id string) (*agiledomain.Sprint, error)
}

type BoardSource interface {
	BoardByID(ctx context.Context, id agiledomain.BoardID) (*agiledomain.Board, error)
}

type Owners struct {
	sprints SprintSource
	boards  BoardSource
}

func NewOwners(s SprintSource, b BoardSource) Owners { return Owners{sprints: s, boards: b} }

func (o Owners) ProjectOf(ctx context.Context, kind, id string) string {
	switch kind {
	case "sprints":
		if s, err := o.sprints.Peek(ctx, id); err == nil {
			return s.Project()
		}
	case "boards":
		if b, err := o.boards.BoardByID(ctx, agiledomain.BoardID(id)); err == nil {
			return b.Project()
		}
	}
	return ""
}
