package domain

import (
	"errors"
	"slices"
	"time"
)

var (
	ErrInvalidAudience = errors.New("invite chosen people, a project (with its key) or the whole workspace")
	ErrNotAttendee     = errors.New("only attendees answer an invitation")
	ErrInvalidResponse = errors.New("answer going, maybe or declined")
)

// Scope is who a meeting is for.
type Scope string

const (
	ScopePeople    Scope = "people"
	ScopeProject   Scope = "project"
	ScopeWorkspace Scope = "workspace"
)

var scopes = []Scope{ScopePeople, ScopeProject, ScopeWorkspace}

// Audience: chosen people, a project's members (Project) or the workspace's.
type Audience struct {
	Scope   Scope
	Project string
}

// Response is an attendee's answer to the invitation.
type Response string

const (
	Going    Response = "going"
	Maybe    Response = "maybe"
	Declined Response = "declined"
)

func ParseResponse(s string) (Response, error) {
	if r := Response(s); r == Going || r == Maybe || r == Declined {
		return r, nil
	}
	return "", ErrInvalidResponse
}

// ReminderLead is how long before the start attendees are reminded.
const ReminderLead = 15 * time.Minute

// Event is a domain event written to the outbox.
type Event interface{ EventName() string }

// Facts are what every meeting event tells the Notification context.
type Facts struct {
	ID        MeetingID   `json:"id"`
	Workspace WorkspaceID `json:"workspace"`
	Project   string      `json:"project,omitempty"`
	Title     string      `json:"title"`
	Start     time.Time   `json:"start"`
	End       time.Time   `json:"end"`
	Attendees []UserID    `json:"attendees"`
}

func (m *Meeting) facts() Facts {
	return Facts{ID: m.ID, Workspace: m.Workspace, Project: m.Audience.Project, Title: m.Title, Start: m.Start, End: m.End,
		Attendees: slices.Clone(m.Attendees)}
}

type Scheduled struct {
	Facts
	Organizer UserID `json:"organizer"`
}

type Cancelled struct {
	Facts
	By UserID `json:"by"`
}

// Reminder goes to the attendees who did not decline.
type Reminder struct{ Facts }

func (Scheduled) EventName() string { return "meeting.scheduled" }
func (Cancelled) EventName() string { return "meeting.cancelled" }
func (Reminder) EventName() string  { return "meeting.reminder" }

// Respond records an attendee's answer.
func (m *Meeting) Respond(u UserID, r Response) error {
	if !slices.Contains(m.Attendees, u) {
		return ErrNotAttendee
	}
	if _, err := ParseResponse(string(r)); err != nil {
		return err
	}
	m.Responses[u] = r
	return nil
}

// Remind raises the reminder once, within ReminderLead before the start; it reports whether it did.
func (m *Meeting) Remind(now time.Time) bool {
	if m.Reminded || !now.Before(m.Start) || now.Before(m.Start.Add(-ReminderLead)) {
		return false
	}
	m.Reminded = true
	f := m.facts()
	f.Attendees = slices.DeleteFunc(f.Attendees, func(u UserID) bool { return m.Responses[u] == Declined })
	m.events = append(m.events, Reminder{f})
	return true
}

// Cancel tells every attendee the meeting is off; the repository removes it.
func (m *Meeting) Cancel(by UserID) {
	m.events = append(m.events, Cancelled{m.facts(), by})
}

// PullEvents returns and clears the raised events.
func (m *Meeting) PullEvents() []Event {
	e := m.events
	m.events = nil
	return e
}
