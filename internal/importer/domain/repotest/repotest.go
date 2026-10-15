// Package repotest is the contract every MappingRepository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

func Run(t *testing.T, newRepo func(t *testing.T) domain.MappingRepository) {
	t.Run("add and list per project", func(t *testing.T) {
		ctx := context.Background()
		r := newRepo(t)
		for _, m := range []domain.Mapping{mk("KYB", "PROJ-1", "KYB-1"), mk("KYB", "PROJ-2", "KYB-2"), mk("OPS", "PROJ-1", "OPS-1")} {
			if err := r.Add(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
		got, err := r.ForProject(ctx, "KYB")
		if err != nil || len(got) != 2 || got["PROJ-1"] != "KYB-1" || got["PROJ-2"] != "KYB-2" {
			t.Fatalf("KYB = %v, %v", got, err)
		}
		if none, err := r.ForProject(ctx, "NOPE"); err != nil || len(none) != 0 {
			t.Fatalf("empty = %v, %v", none, err)
		}
	})
	t.Run("duplicate external key", func(t *testing.T) {
		ctx := context.Background()
		r := newRepo(t)
		_ = r.Add(ctx, mk("KYB", "PROJ-1", "KYB-1"))
		if err := r.Add(ctx, mk("KYB", "PROJ-1", "KYB-9")); !errors.Is(err, domain.ErrAlreadyImported) {
			t.Fatalf("err = %v", err)
		}
	})
}

func mk(project, external, issue string) domain.Mapping {
	return domain.Mapping{Project: project, ExternalKey: external, IssueKey: issue}
}

// RunLock is the contract of ImportLock adapters; newPair returns two handles on the same lock store, as two
// replicas of the server would have.
func RunLock(t *testing.T, newPair func(t *testing.T) (domain.ImportLock, domain.ImportLock)) {
	t.Run("a held project lock blocks the other replica until released", func(t *testing.T) {
		ctx := context.Background()
		a, b := newPair(t)
		release, err := a.Lock(ctx, "KYB")
		if err != nil {
			t.Fatal(err)
		}
		got := make(chan func(), 1)
		go func() {
			r, err := b.Lock(ctx, "KYB")
			if err != nil {
				t.Error(err)
				r = func() {}
			}
			got <- r
		}()
		select {
		case <-got:
			t.Fatal("second holder got the lock while it was held")
		case <-time.After(100 * time.Millisecond):
		}
		release()
		select {
		case r := <-got:
			r()
		case <-time.After(5 * time.Second):
			t.Fatal("lock not handed over after release")
		}
	})
	t.Run("other projects are independent", func(t *testing.T) {
		ctx := context.Background()
		a, b := newPair(t)
		ra, err := a.Lock(ctx, "KYB")
		if err != nil {
			t.Fatal(err)
		}
		defer ra()
		wait, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		rb, err := b.Lock(wait, "OPS")
		if err != nil {
			t.Fatalf("OPS blocked by KYB: %v", err)
		}
		rb()
	})
	t.Run("waiting ends with the context", func(t *testing.T) {
		ctx := context.Background()
		a, b := newPair(t)
		ra, err := a.Lock(ctx, "KYB")
		if err != nil {
			t.Fatal(err)
		}
		defer ra()
		wait, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if _, err := b.Lock(wait, "KYB"); err == nil {
			t.Fatal("got a held lock")
		}
		// The lock still works after an abandoned wait.
		ra()
		rb, err := b.Lock(ctx, "KYB")
		if err != nil {
			t.Fatal(err)
		}
		rb()
	})
}
