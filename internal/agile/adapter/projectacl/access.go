// Package projectacl is the anti-corruption layer from Agile to the Project context.
package projectacl

import (
	"context"
	"errors"

	agileapp "github.com/bakhod1r/kyber/internal/agile/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Projects interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Access struct{ projects Projects }

func New(p Projects) *Access { return &Access{projects: p} }

func (a *Access) Authorize(ctx context.Context, actor, project string, write bool) error {
	need := projectdomain.PermRead
	if write {
		need = projectdomain.PermWrite
	}
	err := a.projects.Authorize(ctx, projectdomain.UserID(actor), project, need)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return agileapp.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return agileapp.ErrForbidden
	}
	return err
}
