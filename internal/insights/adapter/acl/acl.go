// Package acl is the Insights context's anti-corruption layer to Issue Tracking,
// Agile, Project (membership) and Identity.
package acl

import (
	"context"
	"errors"

	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/insights/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Authorizer interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
}

type Access struct{ projects Authorizer }

func NewAccess(p Authorizer) Access { return Access{projects: p} }

func (a Access) Authorize(ctx context.Context, actor, project string) error {
	err := a.projects.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermRead)
	if errors.Is(err, projectdomain.ErrProjectNotFound) || errors.Is(err, projectdomain.ErrForbidden) {
		return app.ErrProjectNotFound
	}
	return err
}

type IssueSnapshotter interface {
	Snapshot(ctx context.Context, project string) ([]*issuedomain.Issue, error)
}

type Issues struct{ src IssueSnapshotter }

func NewIssues(src IssueSnapshotter) Issues { return Issues{src: src} }

func (a Issues) Issues(ctx context.Context, project string) ([]app.IssueRow, error) {
	list, err := a.src.Snapshot(ctx, project)
	if err != nil {
		return nil, err
	}
	rows := make([]app.IssueRow, 0, len(list))
	for _, is := range list {
		r := app.IssueRow{Key: is.Key().String(), Type: string(is.Type()), Status: string(is.Status()),
			Priority: string(is.Priority()), Assignee: string(is.Assignee())}
		if p, ok := is.Estimate(); ok {
			v := int(p)
			r.Points = &v
		}
		rows = append(rows, r)
	}
	return rows, nil
}

type SprintSource interface {
	Peek(ctx context.Context, id string) (*agiledomain.Sprint, error)
	AllSprints(ctx context.Context, project string) ([]*agiledomain.Sprint, error)
}

type Sprints struct{ src SprintSource }

func NewSprints(src SprintSource) Sprints { return Sprints{src: src} }

func info(s *agiledomain.Sprint) app.SprintInfo {
	return app.SprintInfo{ID: string(s.ID()), Project: s.Project(), Name: s.Name(), State: string(s.State()),
		Start: s.StartedAt(), End: s.EndsAt(), Completed: s.CompletedAt()}
}

func (a Sprints) Sprint(ctx context.Context, id string) (app.SprintInfo, error) {
	s, err := a.src.Peek(ctx, id)
	if errors.Is(err, agiledomain.ErrSprintNotFound) {
		return app.SprintInfo{}, app.ErrSprintNotFound
	}
	if err != nil {
		return app.SprintInfo{}, err
	}
	return info(s), nil
}

// ClosedSprints returns the newest closed sprints first.
func (a Sprints) ClosedSprints(ctx context.Context, project string, limit int) ([]app.SprintInfo, error) {
	all, err := a.src.AllSprints(ctx, project)
	if err != nil {
		return nil, err
	}
	var out []app.SprintInfo
	for _, s := range all {
		if s.State() == agiledomain.StateClosed && len(out) < limit {
			out = append(out, info(s))
		}
	}
	return out, nil
}

type Users struct{ users identitydomain.Users }

func NewUsers(u identitydomain.Users) Users { return Users{users: u} }

func (a Users) DisplayName(ctx context.Context, id string) (string, error) {
	u, err := a.users.ByID(ctx, identitydomain.UserID(id))
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return "Former member", nil
	}
	if err != nil {
		return "", err
	}
	return u.Name(), nil
}
