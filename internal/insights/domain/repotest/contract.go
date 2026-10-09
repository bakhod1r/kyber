// Package repotest is the contract every insights Repository adapter must satisfy.
package repotest

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/domain"
)

const IssueA = "a0000000-0000-4000-8000-00000000000a"

func Run(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	p := 25
	entries := []domain.Entry{
		{Event: 3, At: t0.Add(2 * time.Hour), Project: "KYB", Issue: IssueA, Kind: domain.KindStatus, From: "todo", To: "done"},
		{Event: 1, At: t0, Project: "KYB", Issue: IssueA, Kind: domain.KindCreated, To: "todo"},
		{Event: 2, At: t0.Add(time.Hour), Project: "KYB", Issue: IssueA, Kind: domain.KindEstimate, ToPoints: &p},
		{Event: 4, At: t0.Add(time.Hour), Project: "OPS", Issue: IssueA, Kind: domain.KindSprint, To: "s-1"},
	}
	t.Run("ordered by time then event, per project, idempotent", func(t *testing.T) {
		repo := newRepo(t)
		if err := repo.Record(ctx, entries...); err != nil {
			t.Fatal(err)
		}
		if err := repo.Record(ctx, entries[0], entries[1]); err != nil { // redelivery
			t.Fatal(err)
		}
		got, err := repo.ForProject(ctx, "KYB")
		if err != nil || len(got) != 3 {
			t.Fatalf("ForProject = %v, %v", got, err)
		}
		if got[0].Event != 1 || got[1].Event != 2 || got[2].Event != 3 {
			t.Fatalf("order = %d %d %d", got[0].Event, got[1].Event, got[2].Event)
		}
		if got[1].ToPoints == nil || *got[1].ToPoints != 25 || got[1].FromPoints != nil || !got[1].At.Equal(t0.Add(time.Hour)) {
			t.Fatalf("estimate entry = %+v", got[1])
		}
		if got[2].From != "todo" || got[2].To != "done" || got[2].Issue != IssueA || got[2].Kind != domain.KindStatus {
			t.Fatalf("status entry = %+v", got[2])
		}
		if ops, _ := repo.ForProject(ctx, "OPS"); len(ops) != 1 || ops[0].To != "s-1" {
			t.Fatalf("OPS = %v", ops)
		}
	})
}
