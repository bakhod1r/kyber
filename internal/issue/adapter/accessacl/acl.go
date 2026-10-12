// Package accessacl is Issue Tracking's anti-corruption layer to the Access context: it hands
// the issue's attributes to the permission scheme and translates decisions into Issue errors.
package accessacl

import (
	"context"
	"errors"

	accessapp "github.com/bakhod1r/kyber/internal/access/app"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type Decider interface {
	Can(ctx context.Context, actor, project string, perm accessdomain.Permission, on *accessapp.Issue) error
}

type Access struct{ access Decider }

func New(a Decider) Access { return Access{access: a} }

func (a Access) Can(ctx context.Context, actor, project string, perm issueapp.Permission, on *domain.Issue) error {
	var attrs *accessapp.Issue
	if on != nil {
		attrs = &accessapp.Issue{Reporter: string(on.Reporter()), Assignee: string(on.Assignee()), Status: string(on.Status())}
	}
	err := a.access.Can(ctx, actor, project, accessdomain.Permission(perm), attrs)
	switch {
	case errors.Is(err, accessapp.ErrNotFound):
		return issueapp.ErrProjectNotFound
	case errors.Is(err, accessapp.ErrForbidden):
		return errors.Join(issueapp.ErrForbidden, err)
	}
	return err
}
