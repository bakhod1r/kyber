package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestHierarchy(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	epic, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Checkout", Type: "epic"})
	story, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Pay by card", Type: "story", Parent: "KYB-1"})
	if err != nil || story.ParentKey() != epic.Key() {
		t.Fatalf("story = %v, %v", story, err)
	}
	if _, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Sub", Type: "subtask"}); !errors.Is(err, domain.ErrInvalidParent) {
		t.Fatalf("orphan sub-task err = %v", err)
	}
	if _, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Sub", Type: "subtask", Parent: "KYB-1"}); !errors.Is(err, domain.ErrInvalidParent) {
		t.Fatalf("sub-task under epic err = %v", err)
	}
	if _, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "x", Type: "task", Parent: "KYB-99"}); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("missing parent err = %v", err)
	}
	if _, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "x", Type: "task", Parent: "OPS-1"}); !errors.Is(err, domain.ErrInvalidParent) {
		t.Fatalf("parent in another project err = %v", err)
	}
	// Rejected commands do not burn issue numbers.
	task, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Refunds", Type: "task"})
	if task.Key().Number() != 3 {
		t.Fatalf("next key = %s", task.Key())
	}
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Card form", Type: "subtask", Parent: "KYB-2"})

	// Move the task under the epic by editing; then finish the story.
	parent := "KYB-1"
	if _, err := s.Edit(ctx, dev, "KYB-3", app.EditIssue{Version: task.Version(), Parent: &parent}); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Transition(ctx, dev, "KYB-2", "done")
	estimate := 5.0
	st, _ := s.Get(ctx, dev, "KYB-2")
	_, _ = s.Edit(ctx, dev, "KYB-2", app.EditIssue{Version: st.Version(), EstimateSet: true, Estimate: &estimate})

	kids, err := s.Children(ctx, viewer, "KYB-1")
	if err != nil || len(kids.Items) != 2 || kids.Done != 1 || kids.DonePoints != 50 || kids.Points != 50 {
		t.Fatalf("epic progress = %+v, %v", kids, err)
	}
	if sub, _ := s.Children(ctx, dev, "KYB-2"); len(sub.Items) != 1 || sub.Items[0].Type() != domain.TypeSubtask {
		t.Fatalf("sub-tasks = %+v", sub)
	}
	empty := ""
	if _, err := s.Edit(ctx, dev, "KYB-4", app.EditIssue{Version: 1, Parent: &empty}); !errors.Is(err, domain.ErrInvalidParent) {
		t.Fatalf("detaching a sub-task err = %v", err)
	}
	if _, err := s.Children(ctx, alien, "KYB-1"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("alien err = %v", err)
	}
}
