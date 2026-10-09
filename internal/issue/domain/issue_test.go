package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestNewIssueKey(t *testing.T) {
	tests := []struct {
		name    string
		project string
		number  int
		want    string
		wantErr error
	}{
		{"valid", "KYB", 12, "KYB-12", nil},
		{"lowercase project", "kyb", 1, "", domain.ErrInvalidProjectKey},
		{"too short", "K", 1, "", domain.ErrInvalidProjectKey},
		{"too long", "ABCDEFGHIJK", 1, "", domain.ErrInvalidProjectKey},
		{"starts with digit", "1AB", 1, "", domain.ErrInvalidProjectKey},
		{"zero number", "KYB", 0, "", domain.ErrInvalidIssueNumber},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := domain.NewIssueKey(tt.project, tt.number)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && key.String() != tt.want {
				t.Fatalf("key = %q, want %q", key.String(), tt.want)
			}
		})
	}
}

func newWorkflow(t *testing.T) *domain.Workflow {
	t.Helper()
	wf, err := domain.NewWorkflow("todo", map[domain.StatusID][]domain.StatusID{
		"todo":        {"in_progress"},
		"in_progress": {"todo", "done"},
		"done":        {"in_progress"},
	})
	if err != nil {
		t.Fatalf("NewWorkflow: %v", err)
	}
	return wf
}

func newIssue(t *testing.T) *domain.Issue {
	t.Helper()
	key, _ := domain.NewIssueKey("KYB", 1)
	is, err := domain.NewIssue(domain.IssueID("i-1"), key, "Login page", domain.TypeTask, newWorkflow(t))
	if err != nil {
		t.Fatalf("NewIssue: %v", err)
	}
	return is
}

func TestNewIssue(t *testing.T) {
	is := newIssue(t)
	if is.Status() != "todo" {
		t.Fatalf("status = %q, want todo", is.Status())
	}
	ev := is.PullEvents()
	if len(ev) != 1 {
		t.Fatalf("events = %d, want 1", len(ev))
	}
	if _, ok := ev[0].(domain.IssueCreated); !ok {
		t.Fatalf("event = %T, want IssueCreated", ev[0])
	}
	if len(is.PullEvents()) != 0 {
		t.Fatal("PullEvents must drain events")
	}
}

func TestNewIssueRejectsBlankTitle(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	_, err := domain.NewIssue("i-1", key, "   ", domain.TypeTask, newWorkflow(t))
	if !errors.Is(err, domain.ErrEmptyTitle) {
		t.Fatalf("err = %v, want ErrEmptyTitle", err)
	}
}

func TestIssueTransition(t *testing.T) {
	wf := newWorkflow(t)
	is := newIssue(t)
	is.PullEvents()

	if err := is.Transition("done", wf); !errors.Is(err, domain.ErrTransitionNotAllowed) {
		t.Fatalf("todo->done err = %v, want ErrTransitionNotAllowed", err)
	}
	if is.Status() != "todo" {
		t.Fatal("status must not change on rejected transition")
	}
	if len(is.PullEvents()) != 0 {
		t.Fatal("rejected transition must not emit events")
	}

	if err := is.Transition("in_progress", wf); err != nil {
		t.Fatalf("todo->in_progress: %v", err)
	}
	ev := is.PullEvents()
	got, ok := ev[0].(domain.IssueTransitioned)
	if !ok || got.From != "todo" || got.To != "in_progress" {
		t.Fatalf("event = %+v", ev[0])
	}
}

func TestNewWorkflowRejectsUnknownTarget(t *testing.T) {
	_, err := domain.NewWorkflow("todo", map[domain.StatusID][]domain.StatusID{"todo": {"ghost"}})
	if !errors.Is(err, domain.ErrUnknownStatus) {
		t.Fatalf("err = %v, want ErrUnknownStatus", err)
	}
}

func TestNewWorkflowRejectsUnknownInitial(t *testing.T) {
	_, err := domain.NewWorkflow("ghost", map[domain.StatusID][]domain.StatusID{"todo": nil})
	if !errors.Is(err, domain.ErrUnknownStatus) {
		t.Fatalf("err = %v, want ErrUnknownStatus", err)
	}
}

func TestIssueAccessors(t *testing.T) {
	is := newIssue(t)
	if is.ID() != "i-1" || is.Key().String() != "KYB-1" || is.Key().Project() != "KYB" ||
		is.Key().Number() != 1 || is.Title() != "Login page" || is.Type() != domain.TypeTask {
		t.Fatalf("unexpected issue state: %+v", is)
	}
}
