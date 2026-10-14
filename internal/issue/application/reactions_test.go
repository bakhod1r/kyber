package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestCommentReactions(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	key := is.Key().String()
	c, err := s.AddComment(ctx, dev, key, "ready for review")
	if err != nil {
		t.Fatal(err)
	}
	id := string(c.ID())
	for _, r := range []struct{ who, emoji string }{{viewer, "👍"}, {dev, "👍"}, {dev, "🚀"}, {dev, "🚀"}} {
		if err := s.React(ctx, r.who, id, r.emoji); err != nil {
			t.Fatalf("%s reacts %s: %v", r.who, r.emoji, err)
		}
	}
	list, err := s.Comments(ctx, viewer, key)
	if err != nil || len(list) != 1 {
		t.Fatalf("comments = %v %v", list, err)
	}
	want := []domain.ReactionCount{{Emoji: "👍", Count: 2, Mine: true}, {Emoji: "🚀", Count: 1}}
	if got := list[0].Reactions; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("reactions seen by viewer = %+v", got)
	}
	if err := s.Unreact(ctx, viewer, id, "👍"); err != nil {
		t.Fatal(err)
	}
	if err := s.Unreact(ctx, viewer, id, "👍"); err != nil {
		t.Fatalf("unreact is idempotent: %v", err)
	}
	list, _ = s.Comments(ctx, viewer, key)
	if got := list[0].Reactions; len(got) != 2 || got[0].Count != 1 || got[0].Mine {
		t.Fatalf("after unreact = %+v", got)
	}

	for _, tc := range []struct {
		name, who, id, emoji string
		want                 error
	}{
		{"emoji outside the set", dev, id, "🙂", domain.ErrInvalidReaction},
		{"unknown comment", dev, "c-404", "👍", domain.ErrCommentNotFound},
	} {
		if err := s.React(ctx, tc.who, tc.id, tc.emoji); !errors.Is(err, tc.want) {
			t.Errorf("%s: react = %v", tc.name, err)
		}
		if err := s.Unreact(ctx, tc.who, tc.id, tc.emoji); !errors.Is(err, tc.want) {
			t.Errorf("%s: unreact = %v", tc.name, err)
		}
	}
	ops, _ := s.Create(ctx, dev, application.CreateIssue{Project: "OPS", Title: "Deploy", Type: "task"})
	hidden, _ := s.AddComment(ctx, dev, ops.Key().String(), "secret")
	if err := s.React(ctx, viewer, string(hidden.ID()), "👍"); err == nil {
		t.Fatal("people who cannot see the issue cannot react")
	}
}
