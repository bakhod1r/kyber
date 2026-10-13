package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

func TestKanbanBacklog(t *testing.T) {
	cols := []domain.Column{{Name: "Backlog", Statuses: []string{"todo"}}, {Name: "Doing", Statuses: []string{"in_progress"}}, {Name: "Done", Statuses: []string{"done"}}}
	b, _ := domain.NewBoard("b-1", "KYB", "Flow", domain.KindKanban, cols)
	if b.KanbanBacklog() || len(b.BacklogStatuses()) != 0 {
		t.Fatal("off by default")
	}
	if err := b.SetKanbanBacklog(true); err != nil || !b.KanbanBacklog() {
		t.Fatalf("enable: %v", err)
	}
	if got := b.BacklogStatuses(); len(got) != 1 || got[0] != "todo" {
		t.Fatalf("backlog statuses = %v", got)
	}
	// Reconfiguring keeps it only while it still makes sense.
	_ = b.Configure("Flow", domain.KindKanban, cols)
	if !b.KanbanBacklog() {
		t.Fatal("kept on a kanban board")
	}
	_ = b.Configure("Flow", domain.KindScrum, cols)
	if b.KanbanBacklog() {
		t.Fatal("a scrum board has no kanban backlog")
	}
	if err := b.SetKanbanBacklog(true); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("scrum: %v", err)
	}
	one, _ := domain.NewBoard("b-2", "KYB", "One", domain.KindKanban, cols[:1])
	if err := one.SetKanbanBacklog(true); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("single column: %v", err)
	}
	if err := one.SetKanbanBacklog(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
}
