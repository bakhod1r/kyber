package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestImport(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	first, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Existing", Type: "task"})
	rec.reset()
	created := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

	is, err := s.Import(ctx, "KYB", domain.Imported{Title: "Old bug", Type: domain.TypeBug, Reporter: dev, Status: domain.StatusInProgress, CreatedAt: created, ExternalKey: "PROJ-4"})
	if err != nil {
		t.Fatal(err)
	}
	if is.Key().String() != "KYB-2" || is.Status() != domain.StatusInProgress || first.Rank() >= is.Rank() {
		t.Fatalf("imported = %s %s rank %v after %v", is.Key(), is.Status(), is.Rank(), first.Rank())
	}
	if ev := rec.all(); len(ev) != 1 || ev[0].EventName() != "issue.imported" {
		t.Fatalf("events = %+v", ev)
	}
	if _, err := s.Import(ctx, "NOPE", domain.Imported{Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo}); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("unknown project err = %v", err)
	}
	// Invalid input must not burn an issue number.
	if _, err := s.Import(ctx, "KYB", domain.Imported{Title: " ", Type: domain.TypeTask, Status: domain.StatusTodo}); !errors.Is(err, domain.ErrEmptyTitle) {
		t.Fatal(err)
	}
	next, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "After", Type: "task"})
	if next.Key().String() != "KYB-3" {
		t.Fatalf("next key = %s", next.Key())
	}
}
