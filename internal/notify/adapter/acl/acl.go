// Package acl is the Notification context's anti-corruption layer to Issue Tracking,
// Project (membership) and Identity (users).
package acl

import (
	"context"
	"errors"

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
		Reporter: string(is.Reporter()), Assignee: string(is.Assignee())}, nil
}

type Authorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Members struct{ projects Authorizer }

func NewMembers(p Authorizer) Members { return Members{projects: p} }

func (a Members) IsMember(ctx context.Context, project, user string) (bool, error) {
	err := a.projects.Authorize(ctx, projectdomain.UserID(user), project, projectdomain.PermRead)
	if errors.Is(err, projectdomain.ErrProjectNotFound) {
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
