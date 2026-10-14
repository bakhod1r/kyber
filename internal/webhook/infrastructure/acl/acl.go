// Package acl is the Webhook context's anti-corruption layer to Project (webhooks are managed by
// project administrators: ADMINISTER_PROJECTS) and Issue Tracking (development links follow the
// issue's browse permission, security levels included).
package acl

import (
	"context"
	"errors"

	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/webhook/application"
)

type ProjectAuthorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Projects struct{ src ProjectAuthorizer }

func NewProjects(src ProjectAuthorizer) Projects { return Projects{src: src} }

func (a Projects) AuthorizeAdmin(ctx context.Context, actor, project string) error {
	err := a.src.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermAdmin)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}

type IssueAuthorizer interface {
	Authorize(ctx context.Context, actor, rawKey string, perm issueapp.Permission) (*issuedomain.Issue, error)
}

type Issues struct{ src IssueAuthorizer }

func NewIssues(src IssueAuthorizer) Issues { return Issues{src: src} }

func (a Issues) Authorize(ctx context.Context, actor, issue string) (string, error) {
	is, err := a.src.Authorize(ctx, actor, issue, issueapp.PermBrowse)
	switch {
	case err == nil:
		return is.Key().String(), nil
	case errors.Is(err, issueapp.ErrForbidden), errors.Is(err, issueapp.ErrProjectNotFound), errors.Is(err, issueapp.ErrInvalidKey),
		errors.Is(err, issuedomain.ErrIssueNotFound):
		return "", application.ErrIssueNotFound
	}
	return "", err
}
