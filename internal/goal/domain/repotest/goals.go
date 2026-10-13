// Package repotest is the contract every goal repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/goal/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Owner and workspace the factory must create.
const (
	Owner          = "71000000-0000-4000-8000-000000000001"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ab"
)

func id(n int) domain.GoalID {
	return domain.GoalID("72000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, project, name, target string) *domain.Goal {
		g, err := domain.NewGoal(id(n), project, Owner, name, target, "type = bug")
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, g); err != nil || g.Version != 1 {
			t.Fatalf("save %s: %v", name, err)
		}
		return g
	}
	late := mk(1, "KYB", "Late", "2027-03-01")
	mk(2, "KYB", "Open-ended", "")
	mk(3, "KYB", "Soon", "2026-11-01")
	mk(4, "OPS", "Elsewhere", "")

	list, err := repo.ForProject(ctx, "KYB")
	if err != nil || len(list) != 3 || list[0].Name != "Soon" || list[1].Name != "Late" || list[2].Name != "Open-ended" {
		t.Fatalf("for project = %v %v", names(list), err)
	}

	got, err := repo.ByID(ctx, late.ID)
	if err != nil || *got != *late {
		t.Fatalf("by id = %+v %v", got, err)
	}
	_ = got.Report(domain.StatusAtRisk)
	_ = got.Redefine("Later", "", "")
	if err := repo.Save(ctx, got); err != nil || got.Version != 2 {
		t.Fatalf("update: %v", err)
	}
	if again, _ := repo.ByID(ctx, late.ID); again.Status != domain.StatusAtRisk || again.Name != "Later" || again.Target != "" || again.JQL != "" {
		t.Fatalf("updated = %+v", again)
	}
	stale := *late // version 1
	if err := repo.Save(ctx, &stale); !errors.Is(err, domain.ErrGoalConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if err := repo.Save(ctx, &domain.Goal{ID: late.ID, Project: "KYB", Owner: Owner, Name: "dup", Status: domain.StatusOnTrack}); !errors.Is(err, domain.ErrGoalConflict) {
		t.Fatalf("duplicate insert: %v", err)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.ByID(other, late.ID); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("other workspace sees the goal: %v", err)
	}
	if list, _ := repo.ForProject(other, "KYB"); len(list) != 0 {
		t.Fatalf("other workspace lists %v", names(list))
	}
	if err := repo.Delete(other, late.ID); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("other workspace deletes: %v", err)
	}
	if _, err := repo.ByID(ctx, "not-an-id"); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if err := repo.Delete(ctx, "not-an-id"); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("delete bad id: %v", err)
	}
	if err := repo.Delete(ctx, late.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByID(ctx, late.ID); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}

func names(list []*domain.Goal) []string {
	out := make([]string, len(list))
	for i, g := range list {
		out[i] = g.Name
	}
	return out
}
