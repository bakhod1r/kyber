package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/adapter/memory"
	"github.com/bakhod1r/kyber/internal/importer/app"
	"github.com/bakhod1r/kyber/internal/importer/domain"
)

type fakeAccess map[string]string // actor → role in KYB

func (f fakeAccess) AuthorizeAdmin(_ context.Context, actor, project string) error {
	role, ok := f[actor]
	switch {
	case project != "KYB" || !ok:
		return app.ErrProjectNotFound
	case role != "admin":
		return app.ErrForbidden
	}
	return nil
}

func (fakeAccess) Members(context.Context, string, string) ([]domain.Member, error) {
	return []domain.Member{{ID: "u-ali", Email: "ali@kyber.dev", Name: "Ali Valiyev"}, {ID: "u-lead", Email: "lead@kyber.dev", Name: "Lead"}}, nil
}

type imported struct {
	reporter string
	item     domain.Item
}

type fakeIssues struct {
	n    int
	got  []imported
	fail string // external key whose import fails
}

func (f *fakeIssues) Import(_ context.Context, project, reporter string, it domain.Item) (string, error) {
	if it.Key == f.fail {
		return "", errors.New("db down")
	}
	f.n++
	f.got = append(f.got, imported{reporter, it})
	return fmt.Sprintf("%s-%d", project, f.n), nil
}

const csv = "Summary,Issue key,Issue Type,Status,Priority,Assignee,Reporter,Created\n" +
	"Login fails,PROJ-1,Bug,Done,Major,Ali Valiyev,lead@kyber.dev,09/Oct/25 1:42 PM\n" +
	"Add SSO,PROJ-2,Story,To Do,Minor,ghost@x.io,nobody,\n" +
	",PROJ-3,Task,To Do,,,,\n"

func setup() (*app.Service, *fakeIssues, *memory.Repository) {
	issues, maps := &fakeIssues{}, memory.NewRepository()
	return app.NewService(app.Deps{Access: fakeAccess{"u-lead": "admin", "u-dev": "member"}, Members: fakeAccess{}, Issues: issues, Mappings: maps}), issues, maps
}

func TestPreviewChangesNothing(t *testing.T) {
	s, issues, maps := setup()
	r, err := s.Import(context.Background(), "u-lead", "KYB", csv, true)
	if err != nil {
		t.Fatal(err)
	}
	if !r.DryRun || len(r.Items) != 2 || len(r.Errors) != 1 || r.Errors[0].Key != "PROJ-3" || r.Items[0].IssueKey != "" {
		t.Fatalf("report = %+v", r)
	}
	if r.Items[0].AssigneeID != "u-ali" || len(r.Items[1].Warnings) == 0 {
		t.Fatalf("mapping = %+v", r.Items)
	}
	if m, _ := maps.ForProject(context.Background(), "KYB"); len(issues.got) != 0 || len(m) != 0 {
		t.Fatal("a preview must not create anything")
	}
}

func TestRunIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, issues, _ := setup()
	r, err := s.Import(ctx, "u-lead", "KYB", csv, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.DryRun || len(r.Items) != 2 || r.Items[0].IssueKey != "KYB-1" || r.Items[1].IssueKey != "KYB-2" {
		t.Fatalf("report = %+v", r)
	}
	if issues.got[0].reporter != "u-lead" || issues.got[1].reporter != "u-lead" {
		t.Fatalf("reporter: matched member, else the importing admin; got %+v", issues.got)
	}
	again, err := s.Import(ctx, "u-lead", "KYB", csv, false)
	if err != nil || len(again.Items) != 0 || strings.Join(again.Skipped, ",") != "PROJ-1,PROJ-2" || issues.n != 2 {
		t.Fatalf("re-run = %+v, %v (created %d)", again, err, issues.n)
	}
}

func TestRunStopsAtFailureAndResumes(t *testing.T) {
	ctx := context.Background()
	s, issues, _ := setup()
	issues.fail = "PROJ-2"
	r, err := s.Import(ctx, "u-lead", "KYB", csv, false)
	if err == nil || len(r.Items) != 1 || r.Items[0].IssueKey != "KYB-1" {
		t.Fatalf("partial = %+v, %v", r, err)
	}
	issues.fail = ""
	r, err = s.Import(ctx, "u-lead", "KYB", csv, false)
	if err != nil || len(r.Items) != 1 || r.Items[0].Key != "PROJ-2" || len(r.Skipped) != 1 {
		t.Fatalf("resume = %+v, %v", r, err)
	}
}

func TestImportGuards(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	cases := []struct {
		actor, project, csv string
		want                error
	}{
		{"u-dev", "KYB", csv, app.ErrForbidden},
		{"u-alien", "KYB", csv, app.ErrProjectNotFound},
		{"u-lead", "KYB", "not,a\njira\"export", app.ErrInvalidCSV},
		{"u-lead", "KYB", "Summary\nx\n", app.ErrInvalidCSV},
		{"u-lead", "KYB", strings.Repeat("x", app.MaxCSVBytes+1), app.ErrTooLarge},
	}
	for _, c := range cases {
		if _, err := s.Import(ctx, c.actor, c.project, c.csv, true); !errors.Is(err, c.want) {
			t.Errorf("%s %q…: err = %v, want %v", c.actor, c.csv[:min(10, len(c.csv))], err, c.want)
		}
	}
}
