package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/calendar/application"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type ProjectAccess interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
	MemberIDs(ctx context.Context, key string) ([]projectdomain.UserID, error)
	WorkspaceMemberIDs(ctx context.Context) ([]projectdomain.UserID, error)
}

type WorkspaceAccess interface {
	Administers(ctx context.Context, workspace, user string) (bool, error)
	MemberIDs(ctx context.Context, workspace string) ([]string, error)
}

// SiteAdmin reports whether a user is a configured administrator (KYBER_ADMIN_EMAILS).
type SiteAdmin func(ctx context.Context, user string) (bool, error)

// Teams decides who may invite a project or the workspace and who that is.
type Teams struct {
	projects   ProjectAccess
	workspaces WorkspaceAccess
	siteAdmin  SiteAdmin
}

func NewTeams(p ProjectAccess, w WorkspaceAccess, siteAdmin SiteAdmin) Teams {
	return Teams{projects: p, workspaces: w, siteAdmin: siteAdmin}
}

// InviteProject: members who may change the project (not viewers).
func (t Teams) InviteProject(ctx context.Context, actor, project string) error {
	err := t.projects.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermWrite)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}

// InviteWorkspace: the workspace's owners and admins, or configured site administrators.
func (t Teams) InviteWorkspace(ctx context.Context, actor string) error {
	admin, err := t.workspaces.Administers(ctx, tenant.Workspace(ctx), actor)
	if err != nil || admin {
		return err
	}
	site, err := t.siteAdmin(ctx, actor)
	if err == nil && !site {
		err = application.ErrForbidden
	}
	return err
}

func ids(us []projectdomain.UserID, err error) ([]string, error) {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = string(u)
	}
	return out, err
}

func (t Teams) ProjectMembers(ctx context.Context, project string) ([]string, error) {
	return ids(t.projects.MemberIDs(ctx, project))
}

// WorkspaceMembers: a named workspace's members; in the default workspace (single-tenant mode,
// no stored members) everyone in any of its projects.
func (t Teams) WorkspaceMembers(ctx context.Context) ([]string, error) {
	members, err := t.workspaces.MemberIDs(ctx, tenant.Workspace(ctx))
	if err != nil || len(members) > 0 {
		return members, err
	}
	return ids(t.projects.WorkspaceMemberIDs(ctx))
}
