package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Status is what someone shows the workspace next to their name (S66): an emoji and a short text until an
// optional expiry, and Do Not Disturb until a time (chat notifications stay in-app only).
type Status struct {
	Workspace WorkspaceID
	User      UserID
	Emoji     string
	Text      string
	ExpiresAt time.Time // zero: until cleared
	DNDUntil  time.Time // zero: not in DND
}

// StatusView is a status as seen at one moment.
type StatusView struct {
	Emoji, Text string
	ExpiresAt   time.Time
	DND         bool
	DNDUntil    time.Time
}

var ErrInvalidStatus = errors.New("a status is an emoji (up to 8 characters) and up to 100 characters of text, expiring in the future")

const MaxStatusText = 100

func NewStatus(ws WorkspaceID, u UserID, emoji, text string, expires, dndUntil, now time.Time) (*Status, error) {
	text = strings.TrimSpace(text)
	emoji = strings.TrimSpace(emoji)
	if utf8.RuneCountInString(text) > MaxStatusText || utf8.RuneCountInString(emoji) > 8 ||
		(!expires.IsZero() && !expires.After(now)) {
		return nil, ErrInvalidStatus
	}
	return &Status{Workspace: ws, User: u, Emoji: emoji, Text: text, ExpiresAt: expires.UTC(), DNDUntil: dndUntil.UTC()}, nil
}

// At is the status shown at now: empty once expired; DND only before DNDUntil.
func (s *Status) At(now time.Time) StatusView {
	var v StatusView
	if s.ExpiresAt.IsZero() || now.Before(s.ExpiresAt) {
		v.Emoji, v.Text, v.ExpiresAt = s.Emoji, s.Text, s.ExpiresAt
	}
	if now.Before(s.DNDUntil) {
		v.DND, v.DNDUntil = true, s.DNDUntil
	}
	return v
}

// Statuses stores statuses.
type Statuses interface {
	Put(ctx context.Context, s *Status) error
	// Of returns the statuses of the given users in ws that have one.
	Of(ctx context.Context, ws WorkspaceID, users []UserID) ([]*Status, error)
}
