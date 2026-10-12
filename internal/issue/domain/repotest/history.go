package repotest

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// HistoryFactory returns an empty history repository.
type HistoryFactory func(t *testing.T) domain.HistoryRepository

func RunHistory(t *testing.T, newRepo HistoryFactory) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	issue := domain.IssueID(uuidFor(1))
	other := domain.IssueID(uuidFor(2))
	entries := []domain.HistoryEntry{
		{Event: 3, Seq: 1, Issue: issue, At: t0.Add(2 * time.Minute), By: Assignee, Field: "priority", From: "medium", To: "high"},
		{Event: 1, Seq: 0, Issue: issue, At: t0, By: Assignee, Field: "created", To: "Login"},
		{Event: 3, Seq: 0, Issue: issue, At: t0.Add(2 * time.Minute), By: Assignee, Field: "title", From: "Login", To: "Login page"},
		{Event: 2, Seq: 0, Issue: other, At: t0.Add(time.Minute), By: "", Field: "status", From: "todo", To: "done"},
	}
	t.Run("record and read oldest first", func(t *testing.T) {
		repo := newRepo(t)
		if err := repo.Record(ctx, entries...); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ForIssue(ctx, issue)
		if err != nil || len(got) != 3 {
			t.Fatalf("history = %+v, %v", got, err)
		}
		if got[0].Field != "created" || got[1].Field != "title" || got[2].Field != "priority" || got[2].From != "medium" ||
			got[2].By != Assignee || !got[1].At.Equal(t0.Add(2*time.Minute)) {
			t.Fatalf("order/values = %+v", got)
		}
		if o, _ := repo.ForIssue(ctx, other); len(o) != 1 || o[0].By != "" {
			t.Fatalf("system entry = %+v", o)
		}
	})
	t.Run("redelivery is idempotent", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.Record(ctx, entries...)
		if err := repo.Record(ctx, entries...); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.ForIssue(ctx, issue); len(got) != 3 {
			t.Fatalf("duplicates: %+v", got)
		}
	})
	t.Run("workspaces are isolated", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.Record(ctx, entries...)
		if got, _ := repo.ForIssue(tenant.With(ctx, "00000000-0000-4000-8000-0000000000aa"), issue); len(got) != 0 {
			t.Fatalf("leaked: %+v", got)
		}
	})
}
