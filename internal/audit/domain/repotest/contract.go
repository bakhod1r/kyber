// Package repotest is the contract every audit repository must satisfy.
package repotest

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Actor is a user id the adapters must accept.
const Actor = "60000000-0000-4000-8000-000000000001"

func Run(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	mk := func(minute int, project, action string) domain.Entry {
		e, _ := domain.NewEntry(Actor, action, "/api/v1/x", project, 200, t0.Add(time.Duration(minute)*time.Minute))
		return e
	}
	t.Run("append and list newest first", func(t *testing.T) {
		repo := newRepo(t)
		for _, e := range []domain.Entry{mk(1, "KYB", "POST a"), mk(3, "KYB", "PUT c"), mk(2, "KYB", "PATCH b"), mk(4, "OPS", "POST d"), mk(5, "", "POST e")} {
			if err := repo.Append(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.ForProject(ctx, "KYB", 10)
		if err != nil || len(got) != 3 || got[0].Action != "PUT c" || got[2].Action != "POST a" {
			t.Fatalf("list = %+v, %v", got, err)
		}
		if got[0].Actor != Actor || got[0].Status != 200 || got[0].Target != "/api/v1/x" || !got[0].At.Equal(t0.Add(3*time.Minute)) {
			t.Fatalf("values = %+v", got[0])
		}
		if lim, _ := repo.ForProject(ctx, "KYB", 2); len(lim) != 2 || lim[1].Action != "PATCH b" {
			t.Fatalf("limit = %+v", lim)
		}
	})
	t.Run("workspaces are isolated", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.Append(ctx, mk(1, "KYB", "POST a"))
		if got, _ := repo.ForProject(tenant.With(ctx, "00000000-0000-4000-8000-0000000000aa"), "KYB", 10); len(got) != 0 {
			t.Fatalf("leaked: %+v", got)
		}
	})
}
