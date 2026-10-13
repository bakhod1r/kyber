package repotest

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Watcher is a second user adapters must accept (the Postgres factory inserts it and Assignee).
const Watcher = "60000000-0000-4000-8000-000000000002"

// WatcherFactory returns an empty watcher repository and two saved issues.
type WatcherFactory func(t *testing.T) (domain.WatcherRepository, [2]*domain.Issue)

func RunWatchers(t *testing.T, newRepo WatcherFactory) {
	ctx := context.Background()
	repo, is := newRepo(t)
	a, b := is[0].ID(), is[1].ID()
	if got, err := repo.Watchers(ctx, a); err != nil || len(got) != 0 {
		t.Fatalf("fresh = %v %v", got, err)
	}
	for _, w := range []struct {
		issue domain.IssueID
		user  string
	}{{a, Assignee}, {a, Watcher}, {a, Assignee}, {b, Watcher}} {
		if err := repo.Watch(ctx, w.issue, domain.UserID(w.user)); err != nil {
			t.Fatalf("watch: %v", err)
		}
	}
	got, _ := repo.Watchers(ctx, a)
	if len(got) != 2 || got[0] != Assignee || got[1] != Watcher {
		t.Fatalf("watchers of a = %v (idempotent, oldest first)", got)
	}
	if err := repo.Unwatch(ctx, a, Assignee); err != nil {
		t.Fatal(err)
	}
	if err := repo.Unwatch(ctx, a, Assignee); err != nil {
		t.Fatalf("unwatch twice: %v", err)
	}
	if got, _ := repo.Watchers(ctx, a); len(got) != 1 || got[0] != Watcher {
		t.Fatalf("after unwatch = %v", got)
	}
	if got, _ := repo.Watchers(ctx, b); len(got) != 1 {
		t.Fatalf("other issue = %v", got)
	}
}
