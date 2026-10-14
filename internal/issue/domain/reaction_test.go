package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestParseEmoji(t *testing.T) {
	for _, tc := range []struct {
		in  string
		err error
	}{
		{"👍", nil}, {"👎", nil}, {"😄", nil}, {"🎉", nil}, {"😕", nil}, {"❤️", nil}, {"🚀", nil}, {"👀", nil},
		{"", domain.ErrInvalidReaction}, {"🙂", domain.ErrInvalidReaction}, {"+1", domain.ErrInvalidReaction},
	} {
		e, err := domain.ParseEmoji(tc.in)
		if !errors.Is(err, tc.err) || (err == nil && string(e) != tc.in) {
			t.Errorf("ParseEmoji(%q) = %q, %v", tc.in, e, err)
		}
	}
	if len(domain.Emojis()) != 8 || domain.Emojis()[0] != "👍" {
		t.Fatalf("emojis = %v", domain.Emojis())
	}
}

func TestTallyReactions(t *testing.T) {
	me, other := domain.UserID("u1"), domain.UserID("u2")
	got := domain.TallyReactions([]domain.Reaction{
		{Comment: "c1", User: other, Emoji: "🚀"},
		{Comment: "c1", User: me, Emoji: "👍"},
		{Comment: "c1", User: other, Emoji: "👍"},
		{Comment: "c2", User: other, Emoji: "👀"},
	}, me)
	c1 := got["c1"]
	if len(c1) != 2 || c1[0] != (domain.ReactionCount{Emoji: "👍", Count: 2, Mine: true}) ||
		c1[1] != (domain.ReactionCount{Emoji: "🚀", Count: 1}) {
		t.Fatalf("c1 = %+v (fixed order, counts, mine)", c1)
	}
	if len(got["c2"]) != 1 || got["c2"][0].Mine {
		t.Fatalf("c2 = %+v", got["c2"])
	}
}
