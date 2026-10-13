package domain

// Event is something members of a conversation hear about live.
type Event interface{ EventName() string }

type (
	MessagePosted  struct{ Message Message }
	MessageEdited  struct{ Message Message }
	MessageDeleted struct{ Message Message }
	MessageReacted struct{ Message Message }
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
func (Typing) EventName() string         { return "typing" }
