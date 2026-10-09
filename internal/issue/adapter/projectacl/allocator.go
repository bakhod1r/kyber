// Package projectacl is the anti-corruption layer from Issue Tracking to the Project context:
// it allocates issue keys and translates membership checks into Issue Tracking errors.
package projectacl

import (
	"context"
	"errors"

	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	projectapp "github.com/bakhod1r/kyber/internal/project/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

// Projects is the subset of the Project application service this layer needs.
type Projects interface {
	NextIssueNumber(ctx context.Context, projectKey string) (int, error)
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

// Users resolves display names (the Project context's view of Identity).
type Users interface {
	ByID(ctx context.Context, id projectdomain.UserID) (projectapp.UserInfo, error)
}

type Adapter struct {
	projects Projects
	users    Users
}

func New(p Projects, u Users) *Adapter { return &Adapter{projects: p, users: u} }

// IsMember reports whether user belongs to the project (any role).
func (a *Adapter) IsMember(ctx context.Context, project, user string) (bool, error) {
	err := a.projects.Authorize(ctx, projectdomain.UserID(user), project, projectdomain.PermRead)
	if errors.Is(err, projectdomain.ErrProjectNotFound) {
		return false, nil
	}
	return err == nil, err
}

// DisplayName returns the user's name, or a neutral placeholder for deleted users.
func (a *Adapter) DisplayName(ctx context.Context, user string) (string, error) {
	u, err := a.users.ByID(ctx, projectdomain.UserID(user))
	if errors.Is(err, projectapp.ErrUnknownUser) {
		return "Deleted user", nil
	}
	return u.Name, err
}

func (a *Adapter) Next(ctx context.Context, project string) (domain.IssueKey, error) {
	n, err := a.projects.NextIssueNumber(ctx, project)
	if err != nil {
		return domain.IssueKey{}, translate(err)
	}
	return domain.NewIssueKey(project, n)
}

func (a *Adapter) Authorize(ctx context.Context, actor, project string, write bool) error {
	need := projectdomain.PermRead
	if write {
		need = projectdomain.PermWrite
	}
	return translate(a.projects.Authorize(ctx, projectdomain.UserID(actor), project, need))
}

func translate(err error) error {
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return issueapp.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return issueapp.ErrForbidden
	}
	return err
}

// IsAssignable: members and admins can be assigned issues; viewers and outsiders cannot.
func (a *Adapter) IsAssignable(ctx context.Context, project, user string) (bool, error) {
	err := a.projects.Authorize(ctx, projectdomain.UserID(user), project, projectdomain.PermWrite)
	if errors.Is(err, projectdomain.ErrProjectNotFound) || errors.Is(err, projectdomain.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}
