package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func str(s string) *string { return &s }

func TestEditIssue(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	rec.reset()

	got, err := s.Edit(ctx, dev, "KYB-1", app.EditIssue{
		Version: is.Version(), Title: str("Login v2"), Description: str("details"), Priority: str("high"),
		AssigneeSet: true, Assignee: viewer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title() != "Login v2" || got.Description() != "details" || got.Priority() != domain.PriorityHigh || got.Assignee() != viewer {
		t.Fatalf("edited = %+v", got)
	}
	names := []string{}
	for _, e := range rec.all() {
		names = append(names, e.EventName())
	}
	if len(names) != 2 || names[0] != "issue.edited" || names[1] != "issue.assigned" {
		t.Fatalf("events = %v", names)
	}

	// Unassign with an explicit null.
	got, err = s.Edit(ctx, dev, "KYB-1", app.EditIssue{Version: got.Version(), AssigneeSet: true, Assignee: ""})
	if err != nil || got.Assignee() != "" {
		t.Fatalf("unassign = %q, %v", got.Assignee(), err)
	}
}

func TestEditIssueRejections(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	v := is.Version()
	cases := []struct {
		name  string
		actor string
		cmd   app.EditIssue
		want  error
	}{
		{"stale version", dev, app.EditIssue{Version: v + 1, Title: str("x")}, domain.ErrConcurrentModification},
		{"bad priority", dev, app.EditIssue{Version: v, Priority: str("urgent")}, domain.ErrInvalidPriority},
		{"blank title", dev, app.EditIssue{Version: v, Title: str(" ")}, domain.ErrEmptyTitle},
		{"non-member assignee", dev, app.EditIssue{Version: v, AssigneeSet: true, Assignee: alien}, app.ErrInvalidAssignee},
		{"viewer", viewer, app.EditIssue{Version: v, Title: str("x")}, app.ErrForbidden},
		{"outsider", alien, app.EditIssue{Version: v, Title: str("x")}, app.ErrProjectNotFound},
	}
	for _, c := range cases {
		if _, err := s.Edit(ctx, c.actor, "KYB-1", c.cmd); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if stored, _ := s.Get(ctx, dev, "KYB-1"); stored.Title() != "Login" || stored.Version() != v {
		t.Fatal("rejected edits must not persist")
	}
}

func TestComments(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	rec.reset()

	c1, err := s.AddComment(ctx, dev, "KYB-1", " First! ")
	if err != nil || c1.Body() != "First!" || c1.Author() != dev {
		t.Fatalf("AddComment = %+v, %v", c1, err)
	}
	_, _ = s.AddComment(ctx, dev, "KYB-1", "Second")
	if ev := rec.all(); len(ev) != 2 || ev[0].EventName() != "comment.added" {
		t.Fatalf("events = %v", ev)
	}

	list, err := s.Comments(ctx, viewer, "KYB-1")
	if err != nil || len(list) != 2 || list[0].Body() != "First!" || list[1].Body() != "Second" {
		t.Fatalf("Comments = %+v, %v", list, err)
	}
	if list[0].AuthorName != "Name of u-dev" {
		t.Fatalf("author name = %q", list[0].AuthorName)
	}

	if _, err := s.AddComment(ctx, dev, "KYB-1", "  "); !errors.Is(err, domain.ErrInvalidCommentBody) {
		t.Fatalf("blank err = %v", err)
	}
	if _, err := s.AddComment(ctx, viewer, "KYB-1", "hi"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer err = %v", err)
	}
	if _, err := s.Comments(ctx, alien, "KYB-1"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
	if _, err := s.AddComment(ctx, dev, "KYB-9", "hi"); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("missing issue err = %v", err)
	}
}
