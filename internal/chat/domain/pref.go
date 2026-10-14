package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"
)

// Level is how much a conversation tells someone (S71): every message, only mentions (default), or nothing.
type Level string

const (
	LevelAll      Level = "all"
	LevelMentions Level = "mentions"
	LevelMuted    Level = "muted"
)

// Pref is one person's settings for one conversation: notification level and their own sidebar section.
type Pref struct {
	Workspace    WorkspaceID
	User         UserID
	Conversation ConversationID
	Level        Level
	Section      string // "" = the default Channels / Direct messages lists
}

var ErrInvalidPref = errors.New("level is all, mentions or muted; a section name is at most 40 characters")

func NewPref(ws WorkspaceID, u UserID, c ConversationID, level Level, section string) (*Pref, error) {
	if level == "" {
		level = LevelMentions
	}
	section = strings.TrimSpace(section)
	if !slices.Contains([]Level{LevelAll, LevelMentions, LevelMuted}, level) || utf8.RuneCountInString(section) > 40 {
		return nil, ErrInvalidPref
	}
	return &Pref{Workspace: ws, User: u, Conversation: c, Level: level, Section: section}, nil
}

// Prefs stores conversation settings.
type Prefs interface {
	Put(ctx context.Context, p *Pref) error
	// Mine lists u's settings in ws.
	Mine(ctx context.Context, ws WorkspaceID, u UserID) ([]*Pref, error)
	// Of lists everyone's settings for a conversation.
	Of(ctx context.Context, c ConversationID) ([]*Pref, error)
}

// ErrInvalidOrder: at most 50 section names of at most 40 characters each.
var ErrInvalidOrder = errors.New("a section order is at most 50 names of at most 40 characters")

// SectionOrder is the order of someone's own sidebar sections (S85); validated and without duplicates.
func SectionOrder(names []string) ([]string, error) {
	if len(names) > 50 {
		return nil, ErrInvalidOrder
	}
	out := []string{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || utf8.RuneCountInString(n) > 40 {
			return nil, ErrInvalidOrder
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out, nil
}

// SectionOrders stores each person's section order per workspace.
type SectionOrders interface {
	PutOrder(ctx context.Context, ws WorkspaceID, u UserID, order []string) error
	Order(ctx context.Context, ws WorkspaceID, u UserID) ([]string, error) // empty when never set
}
