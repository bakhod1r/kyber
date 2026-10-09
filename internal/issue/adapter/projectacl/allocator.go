// Package projectacl is the anti-corruption layer from Issue Tracking to the Project context:
// it allocates issue keys and translates membership checks into Issue Tracking errors.
package projectacl

import (
	"context"
	"errors"

	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

// Projects is the subset of the Project application service this layer needs.
type Projects interface {
	NextIssueNumber(ctx context.Context, projectKey string) (int, error)
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Adapter struct{ projects Projects }

func New(p Projects) *Adapter { return &Adapter{projects: p} }

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
