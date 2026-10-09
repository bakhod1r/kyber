// Package acl is the Importer context's anti-corruption layer to Project and Issue Tracking.
package acl

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/bakhod1r/kyber/internal/importer/app"
	"github.com/bakhod1r/kyber/internal/importer/domain"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	projectapp "github.com/bakhod1r/kyber/internal/project/app"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Projects interface {
	Authorize(ctx context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error
	Members(ctx context.Context, actor projectdomain.UserID, key string) ([]projectapp.MemberInfo, error)
}

type Project struct{ projects Projects }

func NewProject(p Projects) Project { return Project{projects: p} }

func (a Project) AuthorizeAdmin(ctx context.Context, actor, project string) error {
	return translate(a.projects.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermAdmin))
}

func (a Project) Members(ctx context.Context, actor, project string) ([]domain.Member, error) {
	list, err := a.projects.Members(ctx, projectdomain.UserID(actor), project)
	if err != nil {
		return nil, translate(err)
	}
	out := make([]domain.Member, 0, len(list))
	for _, m := range list {
		out = append(out, domain.Member{ID: string(m.ID), Email: m.Email, Name: m.Name})
	}
	return out, nil
}

func translate(err error) error {
	switch {
	case errors.Is(err, projectdomain.ErrProjectNotFound):
		return app.ErrProjectNotFound
	case errors.Is(err, projectdomain.ErrForbidden):
		return app.ErrForbidden
	}
	return err
}

type Importer interface {
	Import(ctx context.Context, project string, in issuedomain.Imported) (*issuedomain.Issue, error)
}

// Issues turns planned rows into Issue Tracking imports.
type Issues struct {
	issues Importer
	now    func() time.Time
}

func NewIssues(i Importer, now func() time.Time) Issues { return Issues{issues: i, now: now} }

func (a Issues) Import(ctx context.Context, project, reporter string, it domain.Item) (string, error) {
	in := issuedomain.Imported{
		Title: it.Summary, Type: issuedomain.IssueType(it.Type), Reporter: issuedomain.UserID(reporter),
		Description: it.Description, Priority: issuedomain.Priority(it.Priority), Assignee: issuedomain.UserID(it.AssigneeID),
		Status: issuedomain.StatusID(it.Status), CreatedAt: it.Created, ResolvedAt: it.Resolved, ExternalKey: it.Key,
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = a.now().UTC()
	}
	if it.Points != nil {
		p := issuedomain.Points(math.Round(*it.Points * 10))
		in.Estimate = &p
	}
	is, err := a.issues.Import(ctx, project, in)
	if err != nil {
		return "", err
	}
	return is.Key().String(), nil
}
