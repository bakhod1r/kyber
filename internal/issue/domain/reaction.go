package domain

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrInvalidReaction = errors.New("reaction must be one of 👍 👎 😄 🎉 😕 ❤️ 🚀 👀")
	ErrCommentNotFound = errors.New("comment not found")
)

// Emoji is one of the fixed reactions people can put on a comment.
type Emoji string

var emojis = []Emoji{"👍", "👎", "😄", "🎉", "😕", "❤️", "🚀", "👀"}

// Emojis lists the allowed reactions in display order.
func Emojis() []Emoji { return slices.Clone(emojis) }

func ParseEmoji(s string) (Emoji, error) {
	if !slices.Contains(emojis, Emoji(s)) {
		return "", ErrInvalidReaction
	}
	return Emoji(s), nil
}

// Reaction is one person's emoji on a comment.
type Reaction struct {
	Comment CommentID
	User    UserID
	Emoji   Emoji
}

// ReactionCount is how many people reacted to a comment with an emoji and whether the viewer did.
type ReactionCount struct {
	Emoji Emoji
	Count int
	Mine  bool
}

// TallyReactions counts reactions per comment, in the fixed emoji order, from viewer's perspective.
func TallyReactions(rs []Reaction, viewer UserID) map[CommentID][]ReactionCount {
	out := map[CommentID][]ReactionCount{}
	for _, e := range emojis {
		for _, r := range rs {
			if r.Emoji != e {
				continue
			}
			list := out[r.Comment]
			if len(list) == 0 || list[len(list)-1].Emoji != e {
				list = append(list, ReactionCount{Emoji: e})
			}
			last := &list[len(list)-1]
			last.Count++
			last.Mine = last.Mine || r.User == viewer
			out[r.Comment] = list
		}
	}
	return out
}

// ReactionRepository stores reactions on comments; reacting and unreacting are idempotent.
type ReactionRepository interface {
	React(ctx context.Context, r Reaction, at time.Time) error
	Unreact(ctx context.Context, r Reaction) error
	// ForComments lists the reactions on the given comments, oldest first.
	ForComments(ctx context.Context, comments []CommentID) ([]Reaction, error)
}
