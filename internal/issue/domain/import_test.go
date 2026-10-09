package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestNewImportedIssue(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 7)
	created := time.Date(2025, 10, 9, 13, 42, 0, 0, time.UTC)
	resolved := created.Add(20 * time.Hour)
	p := domain.Points(55)
	is, err := domain.NewImportedIssue("i-7", key, domain.Imported{
		Title: " Login fails ", Type: domain.TypeBug, Reporter: "u-lead", Description: "Steps", Priority: domain.PriorityHighest,
		Assignee: "u-ali", Estimate: &p, Status: domain.StatusDone, CreatedAt: created, ResolvedAt: &resolved, ExternalKey: "PROJ-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if is.Title() != "Login fails" || is.Status() != domain.StatusDone || is.Assignee() != "u-ali" || is.Priority() != domain.PriorityHighest || is.Description() != "Steps" {
		t.Fatalf("issue = %+v", is)
	}
	if e, ok := is.Estimate(); !ok || e != 55 {
		t.Fatal("estimate")
	}
	ev := is.PullEvents()
	imp, ok := ev[0].(domain.IssueImported)
	if len(ev) != 1 || !ok || imp.EventName() != "issue.imported" || imp.ExternalKey != "PROJ-1" || !imp.CreatedAt.Equal(created) ||
		imp.ResolvedAt == nil || imp.Status != domain.StatusDone || imp.Estimate == nil || *imp.Estimate != 5.5 || imp.Assignee != "u-ali" {
		t.Fatalf("events = %+v (one issue.imported, no per-field events that would notify people)", ev)
	}

	if _, err := domain.NewImportedIssue("i-8", key, domain.Imported{Title: " ", Type: domain.TypeTask, Status: domain.StatusTodo, CreatedAt: created}); !errors.Is(err, domain.ErrEmptyTitle) {
		t.Fatalf("blank title err = %v", err)
	}
	if _, err := domain.NewImportedIssue("i-8", key, domain.Imported{Title: "x", Type: domain.TypeTask, Status: "ghost", CreatedAt: created}); err == nil {
		t.Fatal("unknown status must be rejected")
	}
}
