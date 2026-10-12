// Package acl is the Notification context's anti-corruption layer to Issue Tracking,
// Project (membership) and Identity (users).
package acl

import (
	"context"
	"errors"

	accessapp "github.com/bakhod1r/kyber/internal/access/app"
	accessdomain "github.com/bakhod1r/kyber/internal/access/domain"
	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/notify/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type IssuePeeker interface {
	Peek(ctx context.Context, key string) (*issuedomain.Issue, error)
}

type Issues struct{ peek IssuePeeker }

func NewIssues(p IssuePeeker) Issues { return Issues{peek: p} }

func (a Issues) Lookup(ctx context.Context, key string) (app.IssueInfo, error) {
	is, err := a.peek.Peek(ctx, key)
	if errors.Is(err, issuedomain.ErrIssueNotFound) || errors.Is(err, issueapp.ErrInvalidKey) {
		return app.IssueInfo{}, app.ErrIssueGone
	}
	if err != nil {
		return app.IssueInfo{}, err
	}
	return app.IssueInfo{Project: is.Key().Project(), Title: is.Title(),
		Reporter: string(is.Reporter()), Assignee: string(is.Assignee()), Confidential: is.SecurityLevel() != ""}, nil
}

type Authorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

// IssueReader reads an issue as a user would (Issue Tracking's authorized Get).
type IssueReader interface {
	Get(ctx context.Context, actor, key string) (*issuedomain.Issue, error)
}

type Members struct{ issues IssueReader }

func NewMembers(i IssueReader) Members { return Members{issues: i} }

func (a Members) CanSee(ctx context.Context, user, issueKey string) (bool, error) {
	_, err := a.issues.Get(ctx, user, issueKey)
	if errors.Is(err, issueapp.ErrProjectNotFound) || errors.Is(err, issuedomain.ErrIssueNotFound) ||
		errors.Is(err, issueapp.ErrInvalidKey) {
		return false, nil
	}
	return err == nil, err
}

type Users struct{ users identitydomain.Users }

func NewUsers(u identitydomain.Users) Users { return Users{users: u} }

func (a Users) UserIDByEmail(ctx context.Context, raw string) (string, bool, error) {
	email, err := identitydomain.ParseEmail(raw)
	if err != nil {
		return "", false, nil
	}
	u, err := a.users.ByEmail(ctx, email)
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(u.ID()), true, nil
}

func (a Users) DisplayName(ctx context.Context, id string) (string, error) {
	u, err := a.users.ByID(ctx, identitydomain.UserID(id))
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return "Deleted user", nil
	}
	if err != nil {
		return "", err
	}
	return u.Name(), nil
}

type ProjectWorkspaces interface {
	WorkspaceOf(ctx context.Context, key string) (projectdomain.WorkspaceID, error)
}

// Workspaces resolves a project's workspace through the Project context.
type Workspaces struct{ projects ProjectWorkspaces }

func NewWorkspaces(p ProjectWorkspaces) Workspaces { return Workspaces{projects: p} }

func (a Workspaces) WorkspaceOf(ctx context.Context, project string) (string, error) {
	ws, err := a.projects.WorkspaceOf(ctx, project)
	return string(ws), err
}

// Contacts reaches users by email and by their connected Telegram chat (private chat id ==
// Telegram user id, the subject of their Telegram identity).
type Contacts struct {
	users    identitydomain.Users
	external identitydomain.ExternalIdentities
}

func NewContacts(u identitydomain.Users, ext identitydomain.ExternalIdentities) Contacts {
	return Contacts{users: u, external: ext}
}

func (c Contacts) Email(ctx context.Context, user string) (string, error) {
	u, err := c.users.ByID(ctx, identitydomain.UserID(user))
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(u.Email()), nil
}

func (c Contacts) TelegramChat(ctx context.Context, user string) (string, bool, error) {
	chat, err := c.external.SubjectOf(ctx, identitydomain.UserID(user), identitydomain.ProviderTelegram)
	if errors.Is(err, identitydomain.ErrNotLinked) {
		return "", false, nil
	}
	return chat, err == nil, err
}

// Decider is the Access context (ADR-0003).
type Decider interface {
	Can(ctx context.Context, actor, project string, perm accessdomain.Permission, on *accessapp.Issue) error
}

// Admins: connecting a project's Telegram groups needs Administer Projects.
type Admins struct{ access Decider }

func NewAdmins(a Decider) Admins { return Admins{access: a} }

func (a Admins) Administer(ctx context.Context, actor, project string) error {
	err := a.access.Can(ctx, actor, project, accessdomain.AdministerProjects, nil)
	switch {
	case errors.Is(err, accessapp.ErrNotFound):
		return app.ErrNotFound
	case errors.Is(err, accessapp.ErrForbidden):
		return app.ErrForbidden
	}
	return err
}

type WorkflowSource interface {
	ProjectWorkflow(ctx context.Context, project string) (*issuedomain.Workflow, error)
}

type SprintSource interface {
	Peek(ctx context.Context, id string) (*agiledomain.Sprint, error)
}

// Lookups names statuses (from the project's workflow) and sprints for group messages.
type Lookups struct {
	workflows WorkflowSource
	sprints   SprintSource
}

func NewLookups(w WorkflowSource, s SprintSource) Lookups { return Lookups{workflows: w, sprints: s} }

func (l Lookups) StatusName(ctx context.Context, project, status string) (string, error) {
	wf, err := l.workflows.ProjectWorkflow(ctx, project)
	if err != nil {
		return "", err
	}
	for _, s := range wf.Statuses() {
		if string(s.ID) == status {
			return s.Name, nil
		}
	}
	return status, nil // a status removed since: show its id
}

func (l Lookups) SprintName(ctx context.Context, id string) (string, string, error) {
	sp, err := l.sprints.Peek(ctx, id)
	if err != nil {
		return "", "", err
	}
	return sp.Project(), sp.Name(), nil
}
