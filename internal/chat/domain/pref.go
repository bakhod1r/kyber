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
