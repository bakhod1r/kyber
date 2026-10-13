// Package projectcompat lets the anti-corruption layers of other contexts ask the Access context
// (ADR-0003) while keeping their existing translation of Project errors.
package projectcompat

import (
	"context"
	"errors"

	accessapp "github.com/bakhod1r/kyber/internal/access/application"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	projectapp "github.com/bakhod1r/kyber/internal/project/application"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

// Decider is the Access context's decision service.
type Decider interface {
	Can(ctx context.Context, actor, project string, perm accessdomain.Permission, on *accessapp.Issue) error
}

// Check decides a project-level permission and reports denials as Project errors:
// hidden → ErrProjectNotFound, missing permission → ErrForbidden.
func Check(ctx context.Context, d Decider, actor, project string, perm accessdomain.Permission) error {
	err := d.Can(ctx, actor, project, perm, nil)
	switch {
	case errors.Is(err, accessapp.ErrNotFound):
		return projectdomain.ErrProjectNotFound
	case errors.Is(err, accessapp.ErrForbidden):
		return errors.Join(projectdomain.ErrForbidden, err)
	}
	return err
}

// Gate stands in for the Project service in other contexts' ACLs: their coarse checks are decided
// by the permission scheme instead, every other method is the Project service's own. Read means
// Browse Projects and Admin means Administer Projects; what "write" means differs per context,
// so the caller names it.
type Gate struct {
	*projectapp.Service
	access Decider
	write  accessdomain.Permission
}

func NewGate(service *projectapp.Service, d Decider, write accessdomain.Permission) Gate {
	return Gate{Service: service, access: d, write: write}
}

func (g Gate) Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error {
	perm := accessdomain.BrowseProjects
	switch need {
	case projectdomain.PermWrite:
		perm = g.write
	case projectdomain.PermAdmin:
		perm = accessdomain.AdministerProjects
	}
	return Check(ctx, g.access, string(actor), key, perm)
}
