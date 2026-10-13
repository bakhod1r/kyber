// Package acl is the Search context's anti-corruption layer to Project, Issue Tracking, Agile and
// Identity: it turns their models into what a query sees.
package acl

import (
	"context"
	"errors"

	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	issueapp "github.com/bakhod1r/kyber/internal/issue/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/search/application"
	"github.com/bakhod1r/kyber/internal/search/domain"
)

type ProjectLister interface {
	List(ctx context.Context, actor projectdomain.UserID) ([]*projectdomain.Project, error)
}

type Projects struct{ src ProjectLister }

func NewProjects(src ProjectLister) Projects { return Projects{src: src} }

func (a Projects) Browsable(ctx context.Context, actor string) ([]string, error) {
	list, err := a.src.List(ctx, projectdomain.UserID(actor))
	if err != nil {
		return nil, err
	}
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.Key()
	}
	return out, nil
}

type IssueSource interface {
	List(ctx context.Context, actor, project string, q issueapp.ListQuery) ([]*issuedomain.Issue, error)
	ProjectWorkflow(ctx context.Context, project string) (*issuedomain.Workflow, error)
}

type Issues struct{ src IssueSource }

func NewIssues(src IssueSource) Issues { return Issues{src: src} }

// Facts lists the project's issues the actor may see (Browse and security levels are applied by
// Issue Tracking), as query facts. A project the actor may not browse yields nothing.
func (a Issues) Facts(ctx context.Context, actor, project string) ([]domain.Issue, error) {
	list, err := a.src.List(ctx, actor, project, issueapp.ListQuery{})
	if errors.Is(err, issueapp.ErrProjectNotFound) || errors.Is(err, issueapp.ErrForbidden) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	wf, err := a.src.ProjectWorkflow(ctx, project)
	if err != nil {
		return nil, err
	}
	names := map[issuedomain.StatusID]string{}
	for _, st := range wf.Statuses() {
		names[st.ID] = st.Name
	}
	out := make([]domain.Issue, 0, len(list))
	for _, is := range list {
		f := domain.Issue{ID: string(is.ID()), Key: is.Key().String(), Project: is.Key().Project(), Number: is.Key().Number(),
			Summary: is.Title(), Description: is.Description(), Type: string(is.Type()), Status: string(is.Status()),
			StatusName: names[is.Status()], Category: string(wf.CategoryOf(is.Status())), Priority: string(is.Priority()),
			Assignee: string(is.Assignee()), Reporter: string(is.Reporter()), Resolution: string(is.Resolution())}
		if sp := is.Sprint(); sp != "" {
			f.Sprints = []string{string(sp)}
		}
		for _, v := range is.FixVersions() {
			f.FixVersions = append(f.FixVersions, string(v))
		}
		if p := is.ParentKey(); p.Number() > 0 {
			f.Parent = p.String()
		}
		if p, ok := is.Estimate(); ok {
			v := p.Float()
			f.Points = &v
		}
		out = append(out, f)
	}
	return out, nil
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

type PlanningSource interface {
	AllSprints(ctx context.Context, project string) ([]*agiledomain.Sprint, error)
	AllVersions(ctx context.Context, project string) ([]*agiledomain.Version, error)
}

type Planning struct{ src PlanningSource }

func NewPlanning(src PlanningSource) Planning { return Planning{src: src} }

func (a Planning) Sprints(ctx context.Context, project string) ([]application.Sprint, error) {
	list, err := a.src.AllSprints(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]application.Sprint, len(list))
	for i, sp := range list {
		out[i] = application.Sprint{NamedID: application.NamedID{ID: string(sp.ID()), Name: sp.Name()}, Open: sp.State() != agiledomain.StateClosed}
	}
	return out, nil
}

func (a Planning) Versions(ctx context.Context, project string) ([]application.NamedID, error) {
	list, err := a.src.AllVersions(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]application.NamedID, len(list))
	for i, v := range list {
		out[i] = application.NamedID{ID: string(v.ID()), Name: v.Name()}
	}
	return out, nil
}
