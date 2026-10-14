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
	Audience   Audience
	Responses  map[UserID]Response // attendees who answered
	Reminded   bool                // the reminder before the start was sent
	// Conversation is the chat channel whose huddle the meeting happens in (S76); by id, "" when none.
	Conversation string
	events       []Event
}

// NewMeeting books a meeting with chosen people.
func NewMeeting(id MeetingID, ws WorkspaceID, title string, start, end time.Time, organizer UserID, attendees []UserID) (*Meeting, error) {
	return PlanMeeting(id, ws, title, start, end, organizer, attendees, Audience{Scope: ScopePeople})
}

// PlanMeeting books a meeting for an audience: chosen people, or a project's or the workspace's
// members (attendees are then those members, resolved by the application). Everyone invited is
// told through the Scheduled event.
func PlanMeeting(id MeetingID, ws WorkspaceID, title string, start, end time.Time, organizer UserID, attendees []UserID, aud Audience) (*Meeting, error) {
	title, err := validTime(title, start, end)
	if err != nil {
		return nil, err
	}
	if aud.Scope == "" {
		aud.Scope = ScopePeople
	}
	if !slices.Contains(scopes, aud.Scope) || (aud.Scope == ScopeProject) != (aud.Project != "") {
		return nil, ErrInvalidAudience
	}
	all := append(slices.Clone(attendees), organizer)
	slices.Sort(all)
	m := &Meeting{ID: id, Workspace: ws, Title: title, Start: start.UTC(), End: end.UTC(), Organizer: organizer,
		Attendees: slices.Compact(all), Audience: aud, Responses: map[UserID]Response{}}
	m.events = append(m.events, Scheduled{m.facts(), m.Organizer})
	return m, nil
}

// validTime checks a meeting's title and time; it returns the trimmed title.
func validTime(title string, start, end time.Time) (string, error) {
	if title = strings.TrimSpace(title); title == "" {
		return "", ErrEmptyTitle
	}
	if start.IsZero() || !end.After(start) || end.Sub(start) > MaxDuration {
		return "", ErrInvalidTime
	}
	return title, nil
}

// Reschedule moves the meeting (and may rename it); the attendees are told through the
// Rescheduled event and reminded again before the new start.
func (m *Meeting) Reschedule(title string, start, end time.Time) error {
	title, err := validTime(title, start, end)
	if err != nil {
		return err
	}
	m.Title, m.Start, m.End, m.Reminded = title, start.UTC(), end.UTC(), false
	m.events = append(m.events, Rescheduled{m.facts(), m.Organizer})
	return nil
}

// Blocks reports whether the meeting keeps u busy at some point in [from, to).
func (m *Meeting) Blocks(u UserID, from, to time.Time) bool {
	return slices.Contains(m.Attendees, u) && from.Before(m.End) && m.Start.Before(to)
}

// ErrAlreadyReminded: another replica sent the reminder first.
var ErrAlreadyReminded = errors.New("meeting reminder already sent")

// Repository stores meetings; saving writes their raised events to the outbox in the same
// transaction.
type Repository interface {
	Create(ctx context.Context, m *Meeting) error
	// Cancel removes the meeting (Cancelled raised); ErrMeetingNotFound if it is gone.
	Cancel(ctx context.Context, m *Meeting) error
	ByID(ctx context.Context, id MeetingID) (*Meeting, error)
	// ForUser lists the user's meetings overlapping [from, to), ordered by start.
	ForUser(ctx context.Context, u UserID, from, to time.Time) ([]*Meeting, error)
	// Respond stores u's answer from m.Responses.
	Respond(ctx context.Context, m *Meeting, u UserID) error
	// Reschedule stores m's title and time, clears its reminder and writes Rescheduled;
	// ErrMeetingNotFound if it is gone.
	Reschedule(ctx context.Context, m *Meeting) error
	// DueForReminder lists meetings of every workspace starting in [from, to) not yet reminded.
	DueForReminder(ctx context.Context, from, to time.Time) ([]*Meeting, error)
	// SaveReminder marks m reminded and writes its Reminder; ErrAlreadyReminded if it was.
	SaveReminder(ctx context.Context, m *Meeting) error
}
