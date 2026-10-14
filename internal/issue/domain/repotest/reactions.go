package repotest

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// ReactionFactory returns an empty reaction repository and two saved comments.
type ReactionFactory func(t *testing.T) (domain.ReactionRepository, [2]domain.CommentID)

func RunReactions(t *testing.T, newRepo ReactionFactory) {
	ctx := context.Background()
	repo, cs := newRepo(t)
	if got, err := repo.ForComments(ctx, cs[:]); err != nil || len(got) != 0 {
		t.Fatalf("fresh = %v %v", got, err)
	}
	if got, err := repo.ForComments(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("no comments = %v %v", got, err)
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i, r := range []domain.Reaction{
		{Comment: cs[0], User: Assignee, Emoji: "🚀"},
		{Comment: cs[0], User: Watcher, Emoji: "👍"},
		{Comment: cs[0], User: Assignee, Emoji: "🚀"}, // idempotent
		{Comment: cs[0], User: Assignee, Emoji: "👍"},
		{Comment: cs[1], User: Watcher, Emoji: "❤️"},
	} {
		if err := repo.React(ctx, r, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("react %d: %v", i, err)
		}
	}
	got, err := repo.ForComments(ctx, cs[:1])
	want := []domain.Reaction{
		{Comment: cs[0], User: Assignee, Emoji: "🚀"},
		{Comment: cs[0], User: Watcher, Emoji: "👍"},
		{Comment: cs[0], User: Assignee, Emoji: "👍"},
	}
	if err != nil || len(got) != len(want) {
		t.Fatalf("reactions = %v %v", got, err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reactions[%d] = %v, want %v (oldest first)", i, got[i], want[i])
		}
	}
	gone := domain.Reaction{Comment: cs[0], User: Assignee, Emoji: "🚀"}
	for range 2 {
		if err := repo.Unreact(ctx, gone); err != nil {
			t.Fatalf("unreact: %v", err)
		}
	}
	if err := repo.Unreact(ctx, domain.Reaction{Comment: "not-a-uuid", User: "nobody", Emoji: "👍"}); err != nil {
		t.Fatalf("unreact unknown: %v", err)
	}
	if got, _ := repo.ForComments(ctx, cs[:]); len(got) != 3 || got[2].Emoji != "❤️" {
		t.Fatalf("after unreact = %v", got)
	}
}
