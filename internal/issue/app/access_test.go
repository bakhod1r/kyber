package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/app"
)

// ADR-0003 attribute rules through the default scheme: "edit own issue" for reporters.
func TestOwnIssueRules(t *testing.T) {
	ctx := context.Background()
	s, _, roles := setupWithAccess()
	// dev reports KYB-1 as a member; later dev is a viewer of the project (role downgraded).
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Mine", Type: "task"})
	_, _ = s.Create(ctx, lead, app.CreateIssue{Project: "KYB", Title: "Theirs", Type: "task"})
	roles["KYB"][dev] = "r"
	demoted := s

	mine, _ := demoted.Get(ctx, dev, "KYB-1")
	title := "Mine, edited"
	if _, err := demoted.Edit(ctx, dev, "KYB-1", app.EditIssue{Version: mine.Version(), Title: &title}); err != nil {
		t.Fatalf("viewer editing own issue: %v", err)
	}
	theirs, _ := demoted.Get(ctx, dev, "KYB-2")
	if _, err := demoted.Edit(ctx, dev, "KYB-2", app.EditIssue{Version: theirs.Version(), Title: &title}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer editing others' issue err = %v", err)
	}
	mine, _ = demoted.Get(ctx, dev, "KYB-1")
	if _, err := demoted.Edit(ctx, dev, "KYB-1", app.EditIssue{Version: mine.Version(), AssigneeSet: true, Assignee: lead}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer assigning err = %v", err)
	}
	if _, err := demoted.Transition(ctx, dev, "KYB-1", "done", ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer transition err = %v", err)
	}
}
