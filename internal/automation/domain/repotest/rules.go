// Package repotest is the contract every automation rule repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/automation/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Owner and the second workspace the factory must create.
const (
	Owner          = "73000000-0000-4000-8000-000000000001"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ac"
)

func id(n int) domain.RuleID {
	return domain.RuleID("74000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

func spec(name string) domain.RuleSpec {
	return domain.RuleSpec{Name: name, Trigger: domain.TriggerTransitioned, ToStatus: "done", Condition: "type = bug",
		Actions: []domain.Action{{Kind: domain.ActionAddLabel, Value: "shipped"}, {Kind: domain.ActionComment, Value: "Thanks!"}}}
}

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, project, name string) *domain.Rule {
		r, err := domain.NewRule(id(n), project, Owner, spec(name))
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, r); err != nil || r.Version != 1 {
			t.Fatalf("save %s: %v (version %d)", name, err, r.Version)
		}
		return r
	}
	b := mk(1, "KYB", "Beta")
	mk(2, "KYB", "Alpha")
	mk(3, "OPS", "Elsewhere")

	t.Run("round trip and order", func(t *testing.T) {
		list, err := repo.ForProject(ctx, "KYB")
		if err != nil || len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "Beta" {
			t.Fatalf("ForProject = %v, %v", list, err)
		}
		got, err := repo.ByID(ctx, b.ID)
		if err != nil || got.Trigger != domain.TriggerTransitioned || got.ToStatus != "done" || got.Condition != "type = bug" ||
			len(got.Actions) != 2 || got.Actions[1] != (domain.Action{Kind: domain.ActionComment, Value: "Thanks!"}) || !got.Enabled || got.Owner != Owner {
			t.Fatalf("ByID = %+v, %v", got, err)
		}
	})
	t.Run("optimistic locking", func(t *testing.T) {
		a, _ := repo.ByID(ctx, b.ID)
		stale, _ := repo.ByID(ctx, b.ID)
		a.Disable()
		if err := repo.Save(ctx, a); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, stale); !errors.Is(err, domain.ErrRuleConflict) {
			t.Fatalf("stale save: %v", err)
		}
		if got, _ := repo.ByID(ctx, b.ID); got.Enabled {
			t.Fatal("disable lost")
		}
	})
	t.Run("workspaces are separate", func(t *testing.T) {
		other := tenant.With(context.Background(), OtherWorkspace)
		if _, err := repo.ByID(other, b.ID); !errors.Is(err, domain.ErrRuleNotFound) {
			t.Fatalf("other workspace sees it: %v", err)
		}
		if list, _ := repo.ForProject(other, "KYB"); len(list) != 0 {
			t.Fatalf("other workspace lists %v", list)
		}
	})
	t.Run("runs, newest first", func(t *testing.T) {
		t0 := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
		for i, ok := range []bool{true, false, true} {
			if err := repo.Record(ctx, domain.Run{Rule: b.ID, Project: "KYB", IssueKey: "KYB-1", At: t0.Add(time.Duration(i) * time.Minute), OK: ok, Detail: "d"}); err != nil {
				t.Fatal(err)
			}
		}
		runs, err := repo.Runs(ctx, "KYB", 2)
		if err != nil || len(runs) != 2 || !runs[0].At.Equal(t0.Add(2*time.Minute)) || runs[1].OK || runs[0].Rule != b.ID || runs[0].IssueKey != "KYB-1" {
			t.Fatalf("Runs = %+v, %v", runs, err)
		}
		if other, _ := repo.Runs(ctx, "OPS", 10); len(other) != 0 {
			t.Fatalf("OPS runs = %v", other)
		}
	})
	t.Run("delete", func(t *testing.T) {
		if err := repo.Delete(ctx, b.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.ByID(ctx, b.ID); !errors.Is(err, domain.ErrRuleNotFound) {
			t.Fatalf("after delete: %v", err)
		}
	})
}
