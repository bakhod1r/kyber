// Package acl is the Docs context's anti-corruption layer to the Project context (roles).
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/docs/application"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type ProjectAuthorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Projects struct{ src ProjectAuthorizer }

func NewProjects(src ProjectAuthorizer) Projects { return Projects{src: src} }

func (a Projects) Authorize(ctx context.Context, actor, project string, change bool) error {
	need := projectdomain.PermRead
	if change {
		need = projectdomain.PermWrite
	}
	err := a.src.Authorize(ctx, projectdomain.UserID(actor), project, need)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}
