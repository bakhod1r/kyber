package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func ptr[T any](v T) *T { return &v }

func TestParsePriority(t *testing.T) {
	for _, p := range []string{"lowest", "low", "medium", "high", "highest"} {
		if got, err := domain.ParsePriority(p); err != nil || string(got) != p {
			t.Fatalf("ParsePriority(%q) = %q, %v", p, got, err)
		}
	}
	if _, err := domain.ParsePriority("urgent"); !errors.Is(err, domain.ErrInvalidPriority) {
		t.Fatalf("err = %v", err)
	}
}

func TestNewIssueDefaults(t *testing.T) {
	is := newIssue(t)
	if is.Reporter() != "u-reporter" {
		t.Fatalf("reporter = %q", is.Reporter())
	}
	if is.Priority() != domain.PriorityMedium || is.Description() != "" || is.Assignee() != "" {
		t.Fatalf("defaults: %q %q %q", is.Priority(), is.Description(), is.Assignee())
	}
}

func TestEdit(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()

	err := is.Edit(domain.Changes{Title: ptr("  New title "), Description: ptr("Steps:\n1. open"), Priority: ptr(domain.PriorityHigh)})
	if err != nil {
		t.Fatal(err)
	}
	if is.Title() != "New title" || is.Description() != "Steps:\n1. open" || is.Priority() != domain.PriorityHigh {
		t.Fatalf("state: %q %q %q", is.Title(), is.Description(), is.Priority())
	}
	ev := is.PullEvents()
	edited, ok := ev[0].(domain.IssueEdited)
	if len(ev) != 1 || !ok || strings.Join(edited.Fields, ",") != "title,description,priority" {
		t.Fatalf("events = %+v", ev)
	}

	// No-op edits emit nothing.
	if err := is.Edit(domain.Changes{Title: ptr("New title")}); err != nil || len(is.PullEvents()) != 0 {
		t.Fatalf("no-op edit: %v", err)
	}
}

func TestEditValidatesAtomically(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()
	cases := []struct {
		c    domain.Changes
		want error
	}{
		{domain.Changes{Title: ptr("ok"), Description: ptr(strings.Repeat("x", 20_001))}, domain.ErrDescriptionTooLong},
		{domain.Changes{Title: ptr(" "), Priority: ptr(domain.PriorityHigh)}, domain.ErrEmptyTitle},
		{domain.Changes{Priority: ptr(domain.Priority("urgent"))}, domain.ErrInvalidPriority},
	}
	for _, c := range cases {
		if err := is.Edit(c.c); !errors.Is(err, c.want) {
			t.Fatalf("Edit err = %v, want %v", err, c.want)
		}
	}
	if is.Title() != "Login page" || is.Priority() != domain.PriorityMedium || len(is.PullEvents()) != 0 {
		t.Fatal("rejected edits must not change anything")
	}
}

func TestAssign(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()
	is.Assign("u-1", "u-actor")
	is.Assign("u-1", "u-actor") // no-op
	is.Assign("", "u-actor")
	ev := is.PullEvents()
	if len(ev) != 2 {
		t.Fatalf("events = %+v", ev)
	}
	a0, b0 := ev[0].(domain.IssueAssigned), ev[1].(domain.IssueAssigned)
	if a0.From != "" || a0.To != "u-1" || a0.By != "u-actor" || b0.From != "u-1" || b0.To != "" || is.Assignee() != "" {
		t.Fatalf("assigned events = %+v %+v", a0, b0)
	}
}

func TestRehydrateSnapshot(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 7)
	is := domain.Rehydrate(domain.Snapshot{
		ID: "i-7", Key: key, Title: "T", Type: domain.TypeBug, Status: domain.StatusDone,
		Description: "d", Priority: domain.PriorityLow, Assignee: "u-2", Reporter: "u-3", Version: 4,
	})
	if is.Version() != 4 || is.Description() != "d" || is.Priority() != domain.PriorityLow || is.Assignee() != "u-2" || is.Reporter() != "u-3" {
		t.Fatalf("%+v", is)
	}
}

func TestNewComment(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	key, _ := domain.NewIssueKey("KYB", 1)
	c, err := domain.NewComment("c-1", "i-1", key, "u-1", "  Looks good \n", at)
	if err != nil || c.Body() != "Looks good" || c.Author() != "u-1" || !c.CreatedAt().Equal(at) || c.IssueID() != "i-1" || c.ID() != "c-1" {
		t.Fatalf("comment = %+v, %v", c, err)
	}
	ev := c.PullEvents()
	added, ok := ev[0].(domain.CommentAdded)
	if len(ev) != 1 || !ok || added.IssueKey.String() != "KYB-1" || added.Author != "u-1" || added.Body != "Looks good" {
		t.Fatalf("events = %+v", ev)
	}
	for _, bad := range []string{"   ", strings.Repeat("x", 10_001)} {
		if _, err := domain.NewComment("c-2", "i-1", key, "u-1", bad, at); !errors.Is(err, domain.ErrInvalidCommentBody) {
			t.Fatalf("err = %v", err)
		}
	}
	r := domain.RehydrateComment("c-1", "i-1", "u-1", "b", at)
	if r.Body() != "b" || len(r.PullEvents()) != 0 {
		t.Fatal("rehydrated comment must not emit events")
	}
}
