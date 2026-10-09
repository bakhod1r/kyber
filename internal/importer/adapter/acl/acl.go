// Package acl is the Importer context's anti-corruption layer to Project and Issue Tracking.
package acl

import (
	"context"
	"errors"
	"math"
	"time"

	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
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

func (a Project) AuthorizeRead(ctx context.Context, actor, project string) error {
	err := a.projects.Authorize(ctx, projectdomain.UserID(actor), project, projectdomain.PermRead)
	if errors.Is(err, projectdomain.ErrForbidden) {
		return app.ErrProjectNotFound
	}
	return translate(err)
}

type Snapshotter interface {
	Snapshot(ctx context.Context, project string) ([]*issuedomain.Issue, error)
}

type Users interface {
	ByID(ctx context.Context, id identitydomain.UserID) (*identitydomain.User, error)
}

// Source reads issues for export, resolving people to emails (what Jira imports match on).
type Source struct {
	issues Snapshotter
	users  Users
}

func NewSource(i Snapshotter, u Users) Source { return Source{issues: i, users: u} }

func (a Source) ExportRows(ctx context.Context, project string) ([]domain.ExportRow, error) {
	list, err := a.issues.Snapshot(ctx, project)
	if err != nil {
		return nil, err
	}
	emails := map[issuedomain.UserID]string{"": ""}
	email := func(id issuedomain.UserID) (string, error) {
		if e, ok := emails[id]; ok {
			return e, nil
		}
		u, err := a.users.ByID(ctx, identitydomain.UserID(id))
		if errors.Is(err, identitydomain.ErrUserNotFound) {
			emails[id] = ""
			return "", nil
		}
		if err != nil {
			return "", err
		}
		emails[id] = string(u.Email())
		return string(u.Email()), nil
	}
	rows := make([]domain.ExportRow, 0, len(list))
	for _, is := range list {
		r := domain.ExportRow{Key: is.Key().String(), Summary: is.Title(), Type: string(is.Type()), Status: string(is.Status()),
			Priority: string(is.Priority()), Description: is.Description()}
		if r.Assignee, err = email(is.Assignee()); err != nil {
			return nil, err
		}
		if r.Reporter, err = email(is.Reporter()); err != nil {
			return nil, err
		}
		if p, ok := is.Estimate(); ok {
			f := p.Float()
			r.Points = &f
		}
		rows = append(rows, r)
	}
	return rows, nil
}
