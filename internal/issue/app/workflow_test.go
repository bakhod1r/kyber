package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func reviewDef() domain.WorkflowDef {
	return domain.WorkflowDef{Initial: "open", Statuses: []domain.Status{
		{ID: "open", Name: "Open", Category: domain.CategoryToDo},
		{ID: "review", Name: "Review", Category: domain.CategoryInProgress},
		{ID: "closed", Name: "Closed", Category: domain.CategoryDone},
	}, Transitions: []domain.Transition{{From: "open", To: "review"}, {From: "review", To: "closed"}}}
}

func TestProjectWorkflowDefaultsToSimplified(t *testing.T) {
	s, _ := setup()
	wf, err := s.Workflow(context.Background(), viewer, "KYB")
	if err != nil || len(wf.Statuses()) != 3 || wf.Initial() != domain.StatusTodo {
		t.Fatalf("wf = %+v, %v", wf, err)
	}
	if _, err := s.Workflow(context.Background(), alien, "KYB"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("alien err = %v", err)
	}
}

func TestSetWorkflowPermissions(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	for who, want := range map[string]error{dev: app.ErrForbidden, viewer: app.ErrForbidden, alien: app.ErrProjectNotFound} {
		if _, err := s.SetWorkflow(ctx, who, "KYB", reviewDef()); !errors.Is(err, want) {
			t.Errorf("%s err = %v, want %v", who, err, want)
		}
	}
	if _, err := s.SetWorkflow(ctx, lead, "KYB", domain.WorkflowDef{Initial: "x"}); !errors.Is(err, domain.ErrInvalidWorkflow) {
		t.Fatalf("invalid def err = %v", err)
	}
}

func TestCustomWorkflowDrivesIssues(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	if _, err := s.SetWorkflow(ctx, lead, "KYB", reviewDef()); err != nil {
		t.Fatal(err)
	}
	is, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	if err != nil || is.Status() != "open" {
		t.Fatalf("created %v in %v", err, is)
	}
	if _, err := s.Transition(ctx, dev, "KYB-1", "closed"); !errors.Is(err, domain.ErrTransitionNotAllowed) {
		t.Fatalf("skip review err = %v", err)
	}
	rec.reset()
	if _, err := s.Transition(ctx, dev, "KYB-1", "review"); err != nil {
		t.Fatal(err)
	}
	if ev := rec.all()[0].(domain.IssueTransitioned); ev.ToCategory != domain.CategoryInProgress {
		t.Fatalf("event = %+v", ev)
	}
	// Other projects keep the default workflow.
	other, _ := s.Create(ctx, dev, app.CreateIssue{Project: "OPS", Title: "Deploy", Type: "task"})
	if other.Status() != domain.StatusTodo {
		t.Fatalf("OPS status = %s", other.Status())
	}
}

// Jira refuses to drop a status that issues still use (they must be migrated first).
func TestSetWorkflowKeepsStatusesInUse(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"}) // in "todo"
	if _, err := s.SetWorkflow(ctx, lead, "KYB", reviewDef()); !errors.Is(err, app.ErrStatusInUse) {
		t.Fatalf("err = %v, want ErrStatusInUse", err)
	}
	def := reviewDef()
	def.Statuses = append(def.Statuses, domain.Status{ID: "todo", Name: "To Do", Category: domain.CategoryToDo})
	wf, err := s.SetWorkflow(ctx, lead, "KYB", def)
	if err != nil || len(wf.Statuses()) != 4 {
		t.Fatalf("wf = %v, %v", wf, err)
	}
	got, _ := s.Workflow(ctx, dev, "KYB")
	if got.Initial() != "open" || !got.IsDone("closed") {
		t.Fatalf("stored = %+v", got.Statuses())
	}
}

// Sprint completion counts any Done-category status as finished (not just "done").
func TestReturnUnfinishedUsesDoneCategory(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if _, err := s.SetWorkflow(ctx, lead, "KYB", reviewDef()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "t", Type: "task"})
		_, _ = s.Edit(ctx, dev, is.Key().String(), app.EditIssue{Version: is.Version(), SprintSet: true, Sprint: "s-1"})
	}
	_, _ = s.Transition(ctx, dev, "KYB-1", "review")
	_, _ = s.Transition(ctx, dev, "KYB-1", "closed")
	completed, returned, err := s.ReturnUnfinished(ctx, "KYB", "s-1")
	if err != nil || completed != 1 || returned != 1 {
		t.Fatalf("ReturnUnfinished = %d, %d, %v", completed, returned, err)
	}
	wf, err := s.ProjectWorkflow(ctx, "KYB")
	if err != nil || !wf.IsDone("closed") {
		t.Fatalf("ProjectWorkflow = %v, %v", wf, err)
	}
}
