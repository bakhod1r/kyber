// Package repotest is the contract every domain.Repository adapter must satisfy.
package repotest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Factory returns an empty repository plus a function reading stored outbox event names.
type Factory func(t *testing.T) (domain.Repository, func() []string)

func Run(t *testing.T, newRepo Factory) {
	ctx := context.Background()
	mk := func(t *testing.T, n int, title string) *domain.Issue {
		t.Helper()
		key, _ := domain.NewIssueKey("KYB", n)
		is, err := domain.NewIssue(domain.IssueID(uuidFor(n)), key, title, domain.TypeTask, domain.DefaultWorkflow())
		if err != nil {
			t.Fatal(err)
		}
		return is
	}

	t.Run("save new then load", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		if err := repo.Save(ctx, is, is.PullEvents()); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ByKey(ctx, is.Key())
		if err != nil {
			t.Fatal(err)
		}
		if got.ID() != is.ID() || got.Title() != "First" || got.Status() != domain.StatusTodo || got.Version() != 1 {
			t.Fatalf("loaded %+v", got)
		}
		if ev := outbox(); len(ev) != 1 || ev[0] != "issue.created" {
			t.Fatalf("outbox = %v", ev)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, _ := newRepo(t)
		key, _ := domain.NewIssueKey("KYB", 42)
		if _, err := repo.ByKey(ctx, key); !errors.Is(err, domain.ErrIssueNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("update bumps version and stores events", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		_ = repo.Save(ctx, is, is.PullEvents())
		loaded, _ := repo.ByKey(ctx, is.Key())
		if err := loaded.Transition(domain.StatusInProgress, domain.DefaultWorkflow()); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, loaded, loaded.PullEvents()); err != nil {
			t.Fatal(err)
		}
		again, _ := repo.ByKey(ctx, is.Key())
		if again.Version() != 2 || again.Status() != domain.StatusInProgress {
			t.Fatalf("loaded %+v", again)
		}
		if ev := outbox(); len(ev) != 2 || ev[1] != "issue.transitioned" {
			t.Fatalf("outbox = %v", ev)
		}
	})

	t.Run("stale version is rejected atomically", func(t *testing.T) {
		repo, outbox := newRepo(t)
		is := mk(t, 1, "First")
		_ = repo.Save(ctx, is, is.PullEvents())
		a, _ := repo.ByKey(ctx, is.Key())
		b, _ := repo.ByKey(ctx, is.Key())
		_ = a.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		if err := repo.Save(ctx, a, a.PullEvents()); err != nil {
			t.Fatal(err)
		}
		_ = b.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		if err := repo.Save(ctx, b, b.PullEvents()); !errors.Is(err, domain.ErrConcurrentModification) {
			t.Fatalf("err = %v, want ErrConcurrentModification", err)
		}
		if ev := outbox(); len(ev) != 2 {
			t.Fatalf("failed save must not write events: %v", ev)
		}
	})

	t.Run("duplicate new key is rejected", func(t *testing.T) {
		repo, _ := newRepo(t)
		a := mk(t, 1, "A")
		_ = repo.Save(ctx, a, nil)
		b := mk(t, 2, "B")
		bKeyDup := domain.Rehydrate(b.ID(), a.Key(), "B", domain.TypeTask, domain.StatusTodo, 0)
		if err := repo.Save(ctx, bKeyDup, nil); !errors.Is(err, domain.ErrConcurrentModification) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("list by project ordered and filtered", func(t *testing.T) {
		repo, _ := newRepo(t)
		for _, n := range []int{3, 1, 2} {
			is := mk(t, n, "t")
			_ = repo.Save(ctx, is, nil)
		}
		other, _ := domain.NewIssueKey("OPS", 1)
		_ = repo.Save(ctx, domain.Rehydrate(domain.IssueID(uuidFor(99)), other, "o", domain.TypeTask, domain.StatusTodo, 0), nil)
		k2, _ := domain.NewIssueKey("KYB", 2)
		two, _ := repo.ByKey(ctx, k2)
		_ = two.Transition(domain.StatusInProgress, domain.DefaultWorkflow())
		_ = repo.Save(ctx, two, nil)

		all, err := repo.ListByProject(ctx, "KYB", nil)
		if err != nil || len(all) != 3 || all[0].Key().Number() != 1 || all[2].Key().Number() != 3 {
			t.Fatalf("all = %v, %v", all, err)
		}
		st := domain.StatusInProgress
		f, _ := repo.ListByProject(ctx, "KYB", &st)
		if len(f) != 1 || f[0].Key().Number() != 2 {
			t.Fatalf("filtered = %v", f)
		}
	})
}

func uuidFor(n int) string {
	const base = "00000000-0000-4000-8000-0000000000"
	return base + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}
