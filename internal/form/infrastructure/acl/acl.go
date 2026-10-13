// Package acl is the Forms context's anti-corruption layer to Project (roles) and Issue Tracking
// (creating the issue a submission becomes).
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/form/application"
	"github.com/bakhod1r/kyber/internal/form/domain"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type ProjectAuthorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Projects struct{ src ProjectAuthorizer }

func NewProjects(src ProjectAuthorizer) Projects { return Projects{src: src} }

var perms = map[application.Need]projectdomain.Permission{
	application.NeedBrowse: projectdomain.PermRead, application.NeedWrite: projectdomain.PermWrite, application.NeedAdmin: projectdomain.PermAdmin,
}

func (a Projects) Authorize(ctx context.Context, actor, project string, need application.Need) error {
	return projectErr(a.src.Authorize(ctx, projectdomain.UserID(actor), project, perms[need]))
}

func projectErr(err error) error {
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}

type IssueCreator interface {
	Create(ctx context.Context, actor string, cmd issueapp.CreateIssue) (*issuedomain.Issue, error)
	Edit(ctx context.Context, actor, rawKey string, cmd issueapp.EditIssue) (*issuedomain.Issue, error)
}

type Issues struct{ src IssueCreator }

func NewIssues(src IssueCreator) Issues { return Issues{src: src} }

// Create files the submission as an issue (the reporter is the actor) and writes its description.
func (a Issues) Create(ctx context.Context, actor, project string, s domain.Submission) (string, error) {
	is, err := a.src.Create(ctx, actor, issueapp.CreateIssue{Project: project, Title: s.Title, Type: s.Type})
	if err != nil {
		return "", issueErr(err)
	}
	desc := s.Description
	if _, err := a.src.Edit(ctx, actor, is.Key().String(), issueapp.EditIssue{Version: is.Version(), Description: &desc}); err != nil {
		return "", issueErr(err)
	}
	return is.Key().String(), nil
}

func issueErr(err error) error {
	switch {
	case errors.Is(err, issueapp.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, issueapp.ErrForbidden):
		return application.ErrForbidden
	case errors.Is(err, issuedomain.ErrEmptyTitle), errors.Is(err, issuedomain.ErrInvalidTitle), errors.Is(err, issuedomain.ErrDescriptionTooLong):
		return errors.Join(domain.ErrInvalidSubmission, err)
	}
	return err
}
