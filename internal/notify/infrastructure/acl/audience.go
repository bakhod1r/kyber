package acl

import (
	"context"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type ProjectMembers interface {
	MemberIDs(ctx context.Context, key string) ([]projectdomain.UserID, error)
	WorkspaceMemberIDs(ctx context.Context) ([]projectdomain.UserID, error)
}

type WorkspaceMembers interface {
	MemberIDs(ctx context.Context, workspace string) ([]string, error)
}

// Audience answers who hears an announcement: a project's members, or a workspace's. The default
// workspace (single-tenant mode) stores no members, so there it is everyone in any of its projects.
type Audience struct {
	projects   ProjectMembers
	workspaces WorkspaceMembers
}

func NewAudience(p ProjectMembers, w WorkspaceMembers) Audience {
	return Audience{projects: p, workspaces: w}
}

func ids(us []projectdomain.UserID, err error) ([]string, error) {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = string(u)
	}
	return out, err
}

func (a Audience) ProjectMembers(ctx context.Context, project string) ([]string, error) {
	return ids(a.projects.MemberIDs(ctx, project))
}

func (a Audience) WorkspaceMembers(ctx context.Context) ([]string, error) {
	members, err := a.workspaces.MemberIDs(ctx, tenant.Workspace(ctx))
	if err != nil || len(members) > 0 {
		return members, err
	}
	return ids(a.projects.WorkspaceMemberIDs(ctx))
}
