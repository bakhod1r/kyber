// Package acl adapts the Audit context and the site-admin list to ssoconnect's ports (ADR-0018).
package acl

import (
	"context"
	"net/http"

	"github.com/bakhod1r/kyber/internal/ssoconnect/application"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// AuditLog is the Audit context's Record.
type AuditLog interface {
	Record(ctx context.Context, actor, action, target, project string, status int) error
}

// Audit writes into the audit log of the workspace the admin acted in. Secrets never appear.
type Audit struct{ Log AuditLog }

func (a Audit) Record(ctx context.Context, actor, action string, c domain.Connection) error {
	state := "enabled"
	if !c.Enabled {
		state = "disabled"
	}
	return a.Log.Record(ctx, actor, action, "sso:"+string(c.ID)+" "+c.Name+" ("+c.Issuer+", "+state+")", "", http.StatusOK)
}

// WorkspaceRoles is the slice of the Workspace context used to authorize connection management.
type WorkspaceRoles interface {
	Administers(ctx context.Context, workspace, user string) (bool, error)
	Owns(ctx context.Context, workspace, user string) (bool, error)
}

// Admins answers with the installation's site admins (KYBER_ADMIN_EMAILS) and workspace roles.
// Any lookup error counts as "no": access is refused rather than granted.
type Admins struct {
	Site       func(ctx context.Context, user string) (bool, error)
	Workspaces WorkspaceRoles
}

func yes(ok bool, err error) bool { return err == nil && ok }

func (a Admins) IsSiteAdmin(ctx context.Context, user string) bool { return yes(a.Site(ctx, user)) }

func (a Admins) Administers(ctx context.Context, workspace, user string) bool {
	return yes(a.Workspaces.Administers(ctx, workspace, user))
}

func (a Admins) Owns(ctx context.Context, workspace, user string) bool {
	return yes(a.Workspaces.Owns(ctx, workspace, user))
}

var (
	_ application.Audit  = Audit{}
	_ application.Admins = Admins{}
)
