// Package acl is the Attachment context's anti-corruption layer to Issue Tracking: attachment
// rights follow the issue's (browse to see, edit to attach, administer to remove others' files).
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/attachment/application"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
)

type IssueAuthorizer interface {
	Authorize(ctx context.Context, actor, rawKey string, perm issueapp.Permission) (*issuedomain.Issue, error)
}

type Issues struct{ src IssueAuthorizer }

func NewIssues(src IssueAuthorizer) Issues { return Issues{src: src} }

var perms = map[application.Need]issueapp.Permission{
	application.View: issueapp.PermBrowse, application.Attach: issueapp.PermEdit, application.Manage: issueapp.PermAdminister,
}

func (a Issues) Authorize(ctx context.Context, actor, issue string, need application.Need) (string, error) {
	is, err := a.src.Authorize(ctx, actor, issue, perms[need])
	switch {
	case err == nil:
		return is.Key().String(), nil
	case errors.Is(err, issueapp.ErrForbidden):
		return "", application.ErrForbidden
	case errors.Is(err, issueapp.ErrProjectNotFound), errors.Is(err, issueapp.ErrInvalidKey), errors.Is(err, issuedomain.ErrIssueNotFound):
		return "", application.ErrIssueNotFound
	}
	return "", err
}
