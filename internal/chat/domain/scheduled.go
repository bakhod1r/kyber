package domain

import (
	"context"
	"errors"
	"time"
)

// ScheduledID identifies a scheduled message; ReminderID a personal reminder.
type (
	ScheduledID string
	ReminderID  string
)

// Scheduled is a message its author wrote to be posted later (S67); only the author sees it before then.
type Scheduled struct {
	ID           ScheduledID
	Workspace    WorkspaceID
	Conversation ConversationID
	Author       UserID
	Body         string
	SendAt       time.Time
	CreatedAt    time.Time
}

var (
	ErrInvalidSchedule   = errors.New("a message can be scheduled from one minute up to 120 days ahead")
	ErrScheduledNotFound = errors.New("scheduled message not found")
)

// MaxScheduleAhead bounds how far ahead a message can be scheduled.
const MaxScheduleAhead = 120 * 24 * time.Hour

func schedule(at, now time.Time) (time.Time, error) {
	if !at.After(now) || at.Sub(now) > MaxScheduleAhead {
		return time.Time{}, ErrInvalidSchedule
	}
	return at.UTC(), nil
}

func NewScheduled(id ScheduledID, ws WorkspaceID, conv ConversationID, author UserID, text string, at, now time.Time) (*Scheduled, error) {
	b, err := body(text)
	if err != nil {
		return nil, err
	}
	at, err = schedule(at, now)
	if err != nil {
		return nil, err
	}
	return &Scheduled{ID: id, Workspace: ws, Conversation: conv, Author: author, Body: b, SendAt: at, CreatedAt: now.UTC()}, nil
}

// Reschedule edits the text and time before it is sent.
func (s *Scheduled) Reschedule(text string, at, now time.Time) error {
	b, err := body(text)
	if err != nil {
		return err
	}
	if at, err = schedule(at, now); err != nil {
		return err
	}
	s.Body, s.SendAt = b, at
	return nil
}

// Message is the message it becomes when sent; its Ref makes a second send of the same schedule a duplicate.
func (s *Scheduled) Message(id MessageID, now time.Time) *Message {
	m := &Message{ID: id, Conversation: s.Conversation, Author: s.Author, Body: s.Body, CreatedAt: now.UTC(), Ref: "scheduled:" + string(s.ID)}
	m.written()
	return m
}

// ScheduledMessages stores scheduled messages until they are sent.
type ScheduledMessages interface {
	Add(ctx context.Context, s *Scheduled) error
	Update(ctx context.Context, s *Scheduled) error
	Delete(ctx context.Context, id ScheduledID) error
	ByID(ctx context.Context, id ScheduledID) (*Scheduled, error) // ErrScheduledNotFound
	// Mine lists u's scheduled messages in ws, soonest first.
	Mine(ctx context.Context, ws WorkspaceID, u UserID) ([]*Scheduled, error)
	// Due lists scheduled messages of every workspace whose time has come, oldest first.
	Due(ctx context.Context, now time.Time) ([]*Scheduled, error)
}

// Reminder is a personal reminder (/remind me …, S67), told to its user as a notification at its time.
type Reminder struct {
	ID           ReminderID
	Workspace    WorkspaceID
	User         UserID
	Conversation ConversationID // where it was set
	Text         string
	At           time.Time
	Reminded     bool
	events       []ReminderDue
}

func NewReminder(id ReminderID, ws WorkspaceID, u UserID, conv ConversationID, text string, at, now time.Time) (*Reminder, error) {
	b, err := body(text)
	if err != nil {
		return nil, err
	}
	if !at.After(now) || at.Sub(now) > MaxScheduleAhead {
		return nil, ErrInvalidReminder
	}
	return &Reminder{ID: id, Workspace: ws, User: u, Conversation: conv, Text: b, At: at.UTC()}, nil
}

func (r *Reminder) Remind(now time.Time) bool {
	if r.Reminded || now.Before(r.At) {
		return false
	}
	r.Reminded = true
	r.events = append(r.events, ReminderDue{User: r.User, Conversation: r.Conversation, Excerpt: r.Text})
	return true
}

func (r *Reminder) PullEvents() []ReminderDue {
	ev := r.events
	r.events = nil
	return ev
}

// Reminders stores personal reminders.
type Reminders interface {
	Add(ctx context.Context, r *Reminder) error
	// Due lists reminders of every workspace whose time has come and that were not sent.
	Due(ctx context.Context, now time.Time) ([]*Reminder, error)
	// MarkReminded records the reminder and its events once; ErrAlreadyReminded if it was.
	MarkReminded(ctx context.Context, r *Reminder) error
}
