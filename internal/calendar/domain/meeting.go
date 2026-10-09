// Package domain is the Calendar context: meetings on the team's time table. During a meeting
// its attendees cannot focus (the Focus context asks through an ACL port).
package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

var (
	ErrEmptyTitle      = errors.New("meeting title must not be empty")
	ErrInvalidTime     = errors.New("a meeting must end after it starts and last at most 8 hours")
	ErrMeetingNotFound = errors.New("meeting not found")
)

const MaxDuration = 8 * time.Hour

type (
	MeetingID   string
	UserID      string
	WorkspaceID string
)

type Meeting struct {
	ID         MeetingID
	Workspace  WorkspaceID
	Title      string
	Start, End time.Time
	Organizer  UserID
	Attendees  []UserID // includes the organizer; sorted, unique
}

func NewMeeting(id MeetingID, ws WorkspaceID, title string, start, end time.Time, organizer UserID, attendees []UserID) (*Meeting, error) {
	if title = strings.TrimSpace(title); title == "" {
		return nil, ErrEmptyTitle
	}
	if start.IsZero() || !end.After(start) || end.Sub(start) > MaxDuration {
		return nil, ErrInvalidTime
	}
	all := append(slices.Clone(attendees), organizer)
	slices.Sort(all)
	return &Meeting{ID: id, Workspace: ws, Title: title, Start: start.UTC(), End: end.UTC(), Organizer: organizer,
		Attendees: slices.Compact(all)}, nil
}

// Blocks reports whether the meeting keeps u busy at some point in [from, to).
func (m *Meeting) Blocks(u UserID, from, to time.Time) bool {
	return slices.Contains(m.Attendees, u) && from.Before(m.End) && m.Start.Before(to)
}

type Repository interface {
	Create(ctx context.Context, m *Meeting) error
	Delete(ctx context.Context, id MeetingID) error // ErrMeetingNotFound
	ByID(ctx context.Context, id MeetingID) (*Meeting, error)
	// ForUser lists the user's meetings overlapping [from, to), ordered by start.
	ForUser(ctx context.Context, u UserID, from, to time.Time) ([]*Meeting, error)
}
