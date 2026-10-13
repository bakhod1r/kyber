package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/form/application"
	"github.com/bakhod1r/kyber/internal/form/domain"
	"github.com/bakhod1r/kyber/internal/form/infrastructure/memory"
)

const (
	admin  = "u-admin"
	member = "u-member"
	viewer = "u-viewer"
	out    = "u-outsider"
)

var rank = map[string]application.Need{admin: application.NeedAdmin, member: application.NeedWrite, viewer: application.NeedBrowse}

type projects struct{}

func (projects) Authorize(_ context.Context, actor, project string, need application.Need) error {
	has, ok := rank[actor]
	switch {
	case project != "KYB" || !ok:
		return application.ErrProjectNotFound
	case need > has:
		return application.ErrForbidden
	}
	return nil
}

type issues struct{ got []domain.Submission }

var errBoom = errors.New("boom")

func (i *issues) Create(_ context.Context, _, project string, s domain.Submission) (string, error) {
	if s.Title == "boom" {
		return "", errBoom
	}
	i.got = append(i.got, s)
	return fmt.Sprintf("%s-%d", project, len(i.got)), nil
}

func TestForms(t *testing.T) {
	ctx := context.Background()
	is := &issues{}
	n := 0
	s := application.NewService(application.Deps{Projects: projects{}, Issues: is, Forms: memory.NewRepository(),
		NewID: func() string { n++; return fmt.Sprintf("f-%d", n) }})
	design := application.Design{Name: "Bug report", IssueType: "bug", Questions: []domain.QuestionInput{{Label: "Steps", Required: true}}}

	f, err := s.Create(ctx, admin, "KYB", design)
	if err != nil || f.Version != 1 {
		t.Fatalf("create = %+v %v", f, err)
	}
	for _, tc := range []struct {
		actor string
		d     application.Design
		want  error
	}{
		{member, design, application.ErrForbidden},
		{out, design, application.ErrProjectNotFound},
		{admin, application.Design{Name: "x", IssueType: "epic"}, domain.ErrInvalidForm},
	} {
		if _, err := s.Create(ctx, tc.actor, "KYB", tc.d); !errors.Is(err, tc.want) {
			t.Errorf("create %s: %v", tc.actor, err)
		}
	}
	if list, err := s.Forms(ctx, viewer, "KYB"); err != nil || len(list) != 1 {
		t.Fatalf("forms = %v %v", list, err)
	}
	if _, err := s.Forms(ctx, out, "KYB"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider lists: %v", err)
	}
	id := string(f.ID)
	if got, err := s.Form(ctx, viewer, id); err != nil || got.Name != "Bug report" {
		t.Fatalf("form = %+v %v", got, err)
	}
	if _, err := s.Form(ctx, out, id); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("outsider reads: %v", err)
	}
	if _, err := s.Form(ctx, admin, "nope"); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("missing form: %v", err)
	}

	key, err := s.Submit(ctx, member, id, "Login fails", map[string]string{"q1": "Click"})
	if err != nil || key != "KYB-1" || is.got[0].Type != "bug" || is.got[0].Title != "Login fails" {
		t.Fatalf("submit = %q %v %+v", key, err, is.got)
	}
	for _, tc := range []struct {
		actor, summary string
		want           error
	}{
		{viewer, "x", application.ErrForbidden},
		{member, "", domain.ErrInvalidSubmission},
		{member, "boom", errBoom},
	} {
		if _, err := s.Submit(ctx, tc.actor, id, tc.summary, map[string]string{"q1": "x"}); !errors.Is(err, tc.want) {
			t.Errorf("submit %s %q: %v", tc.actor, tc.summary, err)
		}
	}

	design.Name = "Bugs"
	if f, err = s.Redesign(ctx, admin, id, 1, design); err != nil || f.Version != 2 || f.Name != "Bugs" {
		t.Fatalf("redesign = %+v %v", f, err)
	}
	for _, tc := range []struct {
		actor   string
		version int
		d       application.Design
		want    error
	}{
		{admin, 1, design, domain.ErrFormConflict},
		{admin, 2, application.Design{}, domain.ErrInvalidForm},
		{member, 2, design, application.ErrForbidden},
	} {
		if _, err := s.Redesign(ctx, tc.actor, id, tc.version, tc.d); !errors.Is(err, tc.want) {
			t.Errorf("redesign %s v%d: %v", tc.actor, tc.version, err)
		}
	}
	if err := s.Delete(ctx, member, id); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member deletes: %v", err)
	}
	if err := s.Delete(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, admin, id); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}
