// Package acl is the Announcements context's anti-corruption layer to Project (roles) and Workspace
// (membership and administrators).
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/announcement/application"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Projects interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Workspaces interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
	Administers(ctx context.Context, workspace, user string) (bool, error)
}

// SiteAdmin reports whether a user is a configured administrator (KYBER_ADMIN_EMAILS).
type SiteAdmin func(ctx context.Context, user string) (bool, error)

type Access struct {
	projects   Projects
	workspaces Workspaces
	siteAdmin  SiteAdmin
}

func NewAccess(p Projects, w Workspaces, siteAdmin SiteAdmin) Access {
	return Access{projects: p, workspaces: w, siteAdmin: siteAdmin}
}

func (a Access) project(ctx context.Context, actor, project string, need projectdomain.Permission) error {
	err := a.projects.Authorize(ctx, projectdomain.UserID(actor), project, need)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}

func (a Access) Read(ctx context.Context, actor, project string) error {
	if project != "" {
		return a.project(ctx, actor, project, projectdomain.PermRead)
	}
	ok, err := a.workspaces.IsMember(ctx, tenant.Workspace(ctx), actor)
	if err == nil && !ok {
		err = application.ErrProjectNotFound
	}
	return err
}

// Publish: project admins for a project; workspace owners/admins or configured site admins for
// the whole workspace.
func (a Access) Publish(ctx context.Context, actor, project string) error {
	if project != "" {
		return a.project(ctx, actor, project, projectdomain.PermAdmin)
	}
	if err := a.Read(ctx, actor, ""); err != nil {
		return err
	}
	admin, err := a.workspaces.Administers(ctx, tenant.Workspace(ctx), actor)
	if err != nil || admin {
		return err
	}
	site, err := a.siteAdmin(ctx, actor)
	if err == nil && !site {
		err = application.ErrForbidden
	}
	return err
}
