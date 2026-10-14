package domain

import (
	"context"
	"errors"
	"time"
)

// Saved is a message someone saved for later (S65), optionally with a time to be reminded of it.
type Saved struct {
	Workspace    WorkspaceID
	User         UserID
	Message      MessageID
	Conversation ConversationID
	SavedAt      time.Time
	RemindAt     time.Time // zero: no reminder
	Reminded     bool
	excerpt      string
	events       []ReminderDue
}

var (
	// ErrAlreadyReminded: another replica sent the reminder first.
	ErrAlreadyReminded = errors.New("reminder already sent")
	ErrInvalidReminder = errors.New("a reminder must be in the future")
)

// ExcerptLen caps the start of the message a reminder carries.
const ExcerptLen = 60

// Save saves m for user; a later save of the same message replaces the reminder time.
func Save(ws WorkspaceID, user UserID, m Message, remindAt, now time.Time) *Saved {
	return &Saved{Workspace: ws, User: user, Message: m.ID, Conversation: m.Conversation, SavedAt: now, RemindAt: remindAt, excerpt: m.Body}
}

// WithExcerpt gives a loaded save the body of its message (for the reminder).
func (s *Saved) WithExcerpt(body string) *Saved { s.excerpt = body; return s }

// Remind is due once its time has come; it records ReminderDue and reports whether it fired.
func (s *Saved) Remind(now time.Time) bool {
	if s.Reminded || s.RemindAt.IsZero() || now.Before(s.RemindAt) {
		return false
	}
	s.Reminded = true
	ex := []rune(s.excerpt)
	if len(ex) > ExcerptLen {
		ex = append(ex[:ExcerptLen-1], '…')
	}
	s.events = append(s.events, ReminderDue{User: s.User, Message: s.Message, Conversation: s.Conversation, Excerpt: string(ex)})
	return true
}

func (s *Saved) PullEvents() []ReminderDue {
	ev := s.events
	s.events = nil
	return ev
}

// ReminderDue is the published event (outbox) for a due reminder: Notification tells the user.
type ReminderDue struct {
	User         UserID         `json:"user_id"`
	Message      MessageID      `json:"message_id"`
	Conversation ConversationID `json:"conversation_id"`
	Excerpt      string         `json:"excerpt"`
}

func (ReminderDue) EventName() string { return "chat.reminder" }

// SavedMessages stores saves.
type SavedMessages interface {
	Put(ctx context.Context, s *Saved) error // insert or replace (resets Reminded)
	Remove(ctx context.Context, ws WorkspaceID, u UserID, m MessageID) error
	// For lists u's saves in ws, most recently saved first.
	For(ctx context.Context, ws WorkspaceID, u UserID) ([]*Saved, error)
	// Due lists saves of every workspace whose reminder time is at or before now and not yet sent, with excerpts.
	Due(ctx context.Context, now time.Time) ([]*Saved, error)
	// MarkReminded records the reminder and its events once; ErrAlreadyReminded if it was.
	MarkReminded(ctx context.Context, s *Saved) error
}
