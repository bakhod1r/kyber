// Package acl is the Goals context's anti-corruption layer to Project (roles) and Search (JQL).
package acl

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/goal/application"
	"github.com/bakhod1r/kyber/internal/goal/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	searchapp "github.com/bakhod1r/kyber/internal/search/application"
	searchdomain "github.com/bakhod1r/kyber/internal/search/domain"
)

type ProjectAuthorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Projects struct{ src ProjectAuthorizer }

func NewProjects(src ProjectAuthorizer) Projects { return Projects{src: src} }

func (a Projects) Authorize(ctx context.Context, actor, project string, change bool) error {
	need := projectdomain.PermRead
	if change {
		need = projectdomain.PermWrite
	}
	err := a.src.Authorize(ctx, projectdomain.UserID(actor), project, need)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return application.ErrForbidden
	}
	return err
}

type Searcher interface {
	Search(ctx context.Context, actor, jql string, limit int) (searchapp.Result, error)
}

type Issues struct{ src Searcher }

func NewIssues(src Searcher) Issues { return Issues{src: src} }

// Measure counts the project's issues the query finds (that the actor may see) and the done ones.
func (a Issues) Measure(ctx context.Context, actor, project, jql string) (domain.Progress, error) {
	if _, err := a.src.Search(ctx, actor, jql, 1); err != nil { // reports errors at the query's own positions
		return domain.Progress{}, invalid(err)
	}
	scoped := fmt.Sprintf("project = %q AND (%s)", project, jql)
	all, err := a.src.Search(ctx, actor, scoped, 1)
	if errors.Is(err, searchdomain.ErrInvalidJQL) {
		return domain.Progress{}, fmt.Errorf("%w: a goal query has no ORDER BY", application.ErrInvalidQuery)
	}
	if err != nil {
		return domain.Progress{}, err
	}
	done, err := a.src.Search(ctx, actor, scoped+" AND statusCategory = done", 1)
	if err != nil {
		return domain.Progress{}, err
	}
	return domain.Progress{Done: done.Total, Total: all.Total}, nil
}

func invalid(err error) error {
	if errors.Is(err, searchdomain.ErrInvalidJQL) {
		return fmt.Errorf("%w: %s", application.ErrInvalidQuery, err.Error())
	}
	return err
}
