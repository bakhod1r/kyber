// Package acl is the Audit context's anti-corruption layer to Project and Identity.
package acl

import (
	"context"
	"errors"

	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/audit/application"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
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
		return application.ErrNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
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

type LinkSource interface {
	ByID(ctx context.Context, id issuedomain.LinkID) (issuedomain.Link, error)
}

type Owners struct {
	sprints SprintSource
	boards  BoardSource
	links   LinkSource
}

func NewOwners(s SprintSource, b BoardSource, l LinkSource) Owners {
	return Owners{sprints: s, boards: b, links: l}
}

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
	case "links":
		if l, err := o.links.ByID(ctx, issuedomain.LinkID(id)); err == nil {
			return l.FromKey.Project()
		}
	}
	return ""
}
