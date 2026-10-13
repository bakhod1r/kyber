// Package acl is the Administration context's anti-corruption layer to Workspace, Project, Identity
// and the outbox.
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/admin/domain"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	workspacedomain "github.com/bakhod1r/kyber/internal/workspace/domain"
)

type Workspaces interface {
	Administers(ctx context.Context, workspace, user string) (bool, error)
	Members(ctx context.Context, workspace string) ([]workspacedomain.Member, error)
	ChangeRole(ctx context.Context, workspace, user, role string) error
}

type Projects interface {
	InWorkspace(ctx context.Context) ([]*projectdomain.Project, error)
	WorkspaceMemberIDs(ctx context.Context) ([]projectdomain.UserID, error)
}

type Users interface {
	ByID(ctx context.Context, id identitydomain.UserID) (*identitydomain.User, error)
}

// SiteAdmin reports whether a user is a configured administrator (KYBER_ADMIN_EMAILS).
type SiteAdmin func(ctx context.Context, user string) (bool, error)

func isDefault(ctx context.Context) bool { return tenant.Workspace(ctx) == tenant.Default }

// Gate: owners and admins of a named workspace; configured administrators of the default one.
type Gate struct {
	workspaces Workspaces
	siteAdmin  SiteAdmin
}

func NewGate(w Workspaces, siteAdmin SiteAdmin) Gate {
	return Gate{workspaces: w, siteAdmin: siteAdmin}
}

func (g Gate) IsAdmin(ctx context.Context, actor string) (bool, error) {
	if isDefault(ctx) {
		return g.siteAdmin(ctx, actor)
	}
	return g.workspaces.Administers(ctx, tenant.Workspace(ctx), actor)
}

// Directory lists the workspace's people. The default workspace stores no roles: there it is
// everyone in any project, configured administrators shown as admins.
type Directory struct {
	workspaces Workspaces
	projects   Projects
	users      Users
	siteAdmin  SiteAdmin
}

func NewDirectory(w Workspaces, p Projects, u Users, siteAdmin SiteAdmin) Directory {
	return Directory{workspaces: w, projects: p, users: u, siteAdmin: siteAdmin}
}

func (d Directory) Members(ctx context.Context) ([]domain.Member, error) {
	var roles []workspacedomain.Member
	if isDefault(ctx) {
		ids, err := d.projects.WorkspaceMemberIDs(ctx)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			role := workspacedomain.RoleMember
			if admin, err := d.siteAdmin(ctx, string(id)); err != nil {
				return nil, err
			} else if admin {
				role = workspacedomain.RoleAdmin
			}
			roles = append(roles, workspacedomain.Member{UserID: workspacedomain.UserID(id), Role: role})
		}
	} else {
		var err error
		if roles, err = d.workspaces.Members(ctx, tenant.Workspace(ctx)); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Member, 0, len(roles))
	for _, m := range roles {
		u, err := d.users.ByID(ctx, identitydomain.UserID(m.UserID))
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Member{ID: string(m.UserID), Name: u.Name(), Email: string(u.Email()), Role: string(m.Role)})
	}
	return out, nil
}

func (d Directory) ChangeRole(ctx context.Context, user, role string) error {
	if isDefault(ctx) {
		return domain.ErrSingleTenant
	}
	err := d.workspaces.ChangeRole(ctx, tenant.Workspace(ctx), user, role)
	switch {
	case errors.Is(err, workspacedomain.ErrLastOwner):
		return domain.ErrLastOwner
	case errors.Is(err, workspacedomain.ErrNotMember):
		return domain.ErrNotMember
	case errors.Is(err, workspacedomain.ErrInvalidRole):
		return domain.ErrInvalidRole
	}
	return err
}

type ProjectList struct{ projects Projects }

func NewProjects(p Projects) ProjectList { return ProjectList{projects: p} }

func (a ProjectList) Projects(ctx context.Context) ([]domain.Project, error) {
	ps, err := a.projects.InWorkspace(ctx)
	out := make([]domain.Project, 0, len(ps))
	for _, p := range ps {
		out = append(out, domain.Project{Key: p.Key(), Name: p.Name(), Members: len(p.Members())})
	}
	return out, err
}

// Store is the outbox's per-workspace dead-letter view.
type Store interface {
	DeadLettersOf(ctx context.Context, workspace string, limit int) ([]outbox.Message, error)
	RetryOf(ctx context.Context, workspace string, id int64) error
}

type DeadLetters struct{ store Store }

func NewDeadLetters(s Store) DeadLetters { return DeadLetters{store: s} }

// limit bounds the list; operators see every workspace's with `kyber outbox dead-letters`.
const limit = 200

func (a DeadLetters) DeadLetters(ctx context.Context) ([]domain.DeadLetter, error) {
	ms, err := a.store.DeadLettersOf(ctx, tenant.Workspace(ctx), limit)
	out := make([]domain.DeadLetter, 0, len(ms))
	for _, m := range ms {
		out = append(out, domain.DeadLetter{ID: m.ID, Event: m.Name, Actor: m.Actor, At: m.At, Attempts: m.Attempts, LastError: m.LastError})
	}
	return out, err
}

func (a DeadLetters) Retry(ctx context.Context, id int64) error {
	err := a.store.RetryOf(ctx, tenant.Workspace(ctx), id)
	if errors.Is(err, outbox.ErrNotDead) {
		return domain.ErrNoDeadLetter
	}
	return err
}
