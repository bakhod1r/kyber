// Package acl is the Automation context's anti-corruption layer to Project (roles), Issue Tracking (actions)
// and Search (JQL conditions).
package acl

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/automation/application"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	searchapp "github.com/bakhod1r/kyber/internal/search/application"
	searchdomain "github.com/bakhod1r/kyber/internal/search/domain"
)

type ProjectAuthorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Projects struct{ src ProjectAuthorizer }

func NewProjects(src ProjectAuthorizer) Projects { return Projects{src: src} }

func (a Projects) Authorize(ctx context.Context, actor, project string, admin bool) error {
	need := projectdomain.PermRead
	if admin {
		need = projectdomain.PermAdmin
	}
	err := a.src.Authorize(ctx, projectdomain.UserID(actor), project, need)
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return application.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden), errors.Is(err, projectdomain.ErrArchived):
		return errors.Join(application.ErrForbidden, err)
	}
	return err
}

// IssueService is the part of Issue Tracking the actions use; its permission checks apply to the rule's owner.
type IssueService interface {
	Get(ctx context.Context, actor, rawKey string) (*issuedomain.Issue, error)
	Transition(ctx context.Context, actor, rawKey, to, resolution string) (*issuedomain.Issue, error)
	Edit(ctx context.Context, actor, rawKey string, cmd issueapp.EditIssue) (*issuedomain.Issue, error)
	AddComment(ctx context.Context, actor, rawKey, body string) (*issuedomain.Comment, error)
}

type Issues struct{ src IssueService }

func NewIssues(src IssueService) Issues { return Issues{src: src} }

func (a Issues) Transition(ctx context.Context, actor, key, status string) error {
	is, err := a.src.Get(ctx, actor, key)
	if err != nil {
		return err
	}
	if string(is.Status()) == status {
		return nil // already there
	}
	_, err = a.src.Transition(ctx, actor, key, status, "")
	return err
}

func (a Issues) Assign(ctx context.Context, actor, key, who string) error {
	is, err := a.src.Get(ctx, actor, key)
	if err != nil {
		return err
	}
	switch who {
	case "reporter":
		who = string(is.Reporter())
	case "unassigned":
		who = ""
	}
	if string(is.Assignee()) == who {
		return nil
	}
	_, err = a.src.Edit(ctx, actor, key, issueapp.EditIssue{Version: is.Version(), AssigneeSet: true, Assignee: who})
	return err
}

func (a Issues) AddLabel(ctx context.Context, actor, key, label string) error {
	is, err := a.src.Get(ctx, actor, key)
	if err != nil {
		return err
	}
	if is.HasLabel(issuedomain.Label(label)) {
		return nil
	}
	labels := []string{label}
	for _, l := range is.Labels() {
		labels = append(labels, string(l))
	}
	_, err = a.src.Edit(ctx, actor, key, issueapp.EditIssue{Version: is.Version(), Labels: &labels})
	return err
}

func (a Issues) Comment(ctx context.Context, actor, key, body string) error {
	_, err := a.src.AddComment(ctx, actor, key, body)
	return err
}

type Searcher interface {
	Search(ctx context.Context, actor, jql string, limit int) (searchapp.Result, error)
}

type Conditions struct{ src Searcher }

func NewConditions(src Searcher) Conditions { return Conditions{src: src} }

func scoped(project, jql string) string { return fmt.Sprintf("project = %q AND (%s)", project, jql) }

func (a Conditions) Validate(ctx context.Context, actor, project, jql string) error {
	if _, err := a.src.Search(ctx, actor, jql, 1); err != nil { // errors at the query's own positions
		return invalid(err)
	}
	if _, err := a.src.Search(ctx, actor, scoped(project, jql), 1); err != nil {
		if errors.Is(err, searchdomain.ErrInvalidJQL) {
			return fmt.Errorf("%w: a condition has no ORDER BY", application.ErrInvalidCondition)
		}
		return err
	}
	return nil
}

func (a Conditions) Matches(ctx context.Context, actor, project, key, jql string) (bool, error) {
	res, err := a.src.Search(ctx, actor, fmt.Sprintf("key = %q AND (%s)", key, jql)+" AND "+fmt.Sprintf("project = %q", project), 1)
	if errors.Is(err, searchdomain.ErrInvalidJQL) {
		return false, invalid(err) // the condition broke after it was saved (a field was removed)
	}
	if err != nil {
		return false, err
	}
	return res.Total > 0, nil
}

func invalid(err error) error {
	if errors.Is(err, searchdomain.ErrInvalidJQL) {
		return fmt.Errorf("%w: %s", application.ErrInvalidCondition, err.Error())
	}
	return err
}
