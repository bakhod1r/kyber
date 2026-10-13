package domain

import "time"

// Event is something members of a conversation hear about live.
type Event interface{ EventName() string }

type (
	MessagePosted  struct{ Message Message }
	MessageEdited  struct{ Message Message }
	MessageDeleted struct{ Message Message }
	MessageReacted struct{ Message Message }
	MessagePinned  struct{ Message Message } // pinned or unpinned
	// ConversationChanged: a channel was renamed, archived or its members or admins changed (S69).
	ConversationChanged struct{ Conversation Conversation }
	// Read: someone read a DM up to At (S68); told to the others in the DM only.
	Read struct {
		Conversation ConversationID
		User         UserID
		At           time.Time
	}
	// Typing is ephemeral: never stored.
	Typing struct {
		Conversation ConversationID
		User         UserID
	}
)

func (MessagePosted) EventName() string  { return "message.posted" }
func (MessageEdited) EventName() string  { return "message.edited" }
func (MessageDeleted) EventName() string { return "message.deleted" }
func (MessageReacted) EventName() string { return "message.reacted" }
func (MessagePinned) EventName() string  { return "message.pinned" }
func (Typing) EventName() string         { return "typing" }
func (Read) EventName() string           { return "conversation.read" }
func (ConversationChanged) EventName() string {
	return "conversation.updated"
}

// MessageWritten is the published event (outbox) for a new message or reply: other contexts
// (Notification: mentions) react to it.
type MessageWritten struct {
	ID           MessageID      `json:"id"`
	Conversation ConversationID `json:"conversation_id"`
	Thread       MessageID      `json:"thread_id"`
	Author       UserID         `json:"author_id"`
	Body         string         `json:"body"`
	Kind         MessageKind    `json:"kind,omitempty"` // e.g. huddle_recording (S77)
}

func (MessageWritten) EventName() string { return "chat.message_posted" }
