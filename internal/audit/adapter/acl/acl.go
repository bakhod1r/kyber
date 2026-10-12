// Package acl is the Audit context's anti-corruption layer to Project and Identity.
package acl

import (
	"context"
	"errors"

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
