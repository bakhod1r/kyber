// Package repotest is the contract every domain.Repository adapter must satisfy.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Assignee is a user ID adapters must accept as an assignee (the Postgres
// test factory inserts this user so the foreign key holds).
const Assignee = "60000000-0000-4000-8000-000000000001"

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
		bKeyDup := domain.Rehydrate(domain.Snapshot{ID: b.ID(), Key: a.Key(), Title: "B", Type: domain.TypeTask, Status: domain.StatusTodo})
		if err := repo.Save(ctx, bKeyDup, nil); !errors.Is(err, domain.ErrConcurrentModification) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("details round-trip", func(t *testing.T) {
		repo, _ := newRepo(t)
		is := mk(t, 1, "First")
		_ = repo.Save(ctx, is, nil)
		loaded, _ := repo.ByKey(ctx, is.Key())
		desc, prio := "Line 1\nLine 2 ✓", domain.PriorityHighest
		if err := loaded.Edit(domain.Changes{Description: &desc, Priority: &prio}); err != nil {
			t.Fatal(err)
		}
		loaded.Assign(domain.UserID(Assignee))
		if err := repo.Save(ctx, loaded, loaded.PullEvents()); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ByKey(ctx, is.Key())
		if got.Description() != desc || got.Priority() != prio || got.Assignee() != domain.UserID(Assignee) {
			t.Fatalf("loaded %q %q %q", got.Description(), got.Priority(), got.Assignee())
		}
		got.Assign("")
		_ = repo.Save(ctx, got, nil)
		if again, _ := repo.ByKey(ctx, is.Key()); again.Assignee() != "" {
			t.Fatalf("unassign not persisted: %q", again.Assignee())
		}
	})

	t.Run("list by project ordered and filtered", func(t *testing.T) {
		repo, _ := newRepo(t)
		for _, n := range []int{3, 1, 2} {
			is := mk(t, n, "t")
			_ = repo.Save(ctx, is, nil)
		}
		other, _ := domain.NewIssueKey("OPS", 1)
		_ = repo.Save(ctx, domain.Rehydrate(domain.Snapshot{ID: domain.IssueID(uuidFor(99)), Key: other, Title: "o", Type: domain.TypeTask, Status: domain.StatusTodo}), nil)
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

// CommentFactory returns an empty comment repository, a saved issue to attach comments to,
// and a function reading outbox event names.
type CommentFactory func(t *testing.T) (domain.CommentRepository, *domain.Issue, func() []string)

func RunComments(t *testing.T, newRepo CommentFactory) {
	ctx := context.Background()
	t.Run("add and list oldest first", func(t *testing.T) {
		repo, is, outbox := newRepo(t)
		base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		for i, body := range []string{"second", "first", "third"} {
			at := base.Add(time.Duration([]int{2, 1, 3}[i]) * time.Minute)
			c, err := domain.NewComment(domain.CommentID(uuidFor(10+i)), is.ID(), is.Key(), domain.UserID(Assignee), body, at)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Add(ctx, c, c.PullEvents()); err != nil {
				t.Fatal(err)
			}
		}
		list, err := repo.ListByIssue(ctx, is.ID())
		if err != nil || len(list) != 3 {
			t.Fatalf("list = %v, %v", list, err)
		}
		if list[0].Body() != "first" || list[2].Body() != "third" || !list[0].CreatedAt().Equal(base.Add(time.Minute)) {
			t.Fatalf("order/time wrong: %q %q %v", list[0].Body(), list[2].Body(), list[0].CreatedAt())
		}
		if list[0].Author() != domain.UserID(Assignee) || list[0].IssueID() != is.ID() {
			t.Fatalf("fields wrong: %+v", list[0])
		}
		if ev := outbox(); len(ev) != 3 || ev[0] != "comment.added" {
			t.Fatalf("outbox = %v", ev)
		}
	})
	t.Run("other issue has no comments", func(t *testing.T) {
		repo, _, _ := newRepo(t)
		list, err := repo.ListByIssue(ctx, domain.IssueID(uuidFor(98)))
		if err != nil || len(list) != 0 {
			t.Fatalf("list = %v, %v", list, err)
		}
	})
}
