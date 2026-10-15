// Package acl adapts the Workspace, Project and Audit contexts to ssosync's ports (S106).
package acl

import (
	"context"
	"net/http"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/ssosync/application"
	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

type (
	// WorkspaceRoles is the slice of the Workspace context group sync uses.
	WorkspaceRoles interface {
		Grant(ctx context.Context, workspace, user, role string) error
		Revoke(ctx context.Context, workspace, user string) error
	}
	// ProjectRoles is the slice of the Project context group sync uses.
	ProjectRoles interface {
		GrantRole(ctx context.Context, key, user, role string) error
		RevokeRole(ctx context.Context, key, user string) error
	}
	// AuditLog is the Audit context's Record.
	AuditLog interface {
		Record(ctx context.Context, actor, action, target, project string, status int) error
	}
)

// Roles runs each change inside its target's workspace (projects are looked up per workspace,
// ADR-0004). Membership of the default workspace is implicit, so it is never granted or revoked.
type Roles struct {
	Workspaces WorkspaceRoles
	Projects   ProjectRoles
}

func (r Roles) Grant(ctx context.Context, t domain.Target, user, role string) error {
	ctx = tenant.With(ctx, t.Workspace)
	if t.Scope == domain.ScopeProject {
		return r.Projects.GrantRole(ctx, t.Project, user, role)
	}
	if t.Workspace == tenant.Default {
		return nil
	}
	return r.Workspaces.Grant(ctx, t.Workspace, user, role)
}

func (r Roles) Revoke(ctx context.Context, t domain.Target, user string) error {
	ctx = tenant.With(ctx, t.Workspace)
	if t.Scope == domain.ScopeProject {
		return r.Projects.RevokeRole(ctx, t.Project, user)
	}
	if t.Workspace == tenant.Default {
		return nil
	}
	return r.Workspaces.Revoke(ctx, t.Workspace, user)
}

// Audit writes into the target workspace's audit log, attributed to the user who signed in.
type Audit struct{ Log AuditLog }

func (a Audit) Record(ctx context.Context, user, action string, t domain.Target, role string) error {
	return a.Log.Record(tenant.With(ctx, t.Workspace), user, action, string(t.Scope)+":"+t.String()+" role="+role, t.Project, http.StatusOK)
}

// Admins answers with the installation's site admins (KYBER_ADMIN_EMAILS).
type Admins func(ctx context.Context, user string) (bool, error)

func (a Admins) IsSiteAdmin(ctx context.Context, user string) bool {
	ok, err := a(ctx, user)
	return err == nil && ok
}

var (
	_ application.Roles  = Roles{}
	_ application.Audit  = Audit{}
	_ application.Admins = Admins(nil)
)
