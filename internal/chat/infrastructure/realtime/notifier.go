// Package realtime publishes Chat events to the members' open tabs (platform/realtime Hub).
package realtime

import (
	"context"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Publisher interface {
	Publish(ws string, users []string, typ string, data any) error
}

type Notifier struct{ hub Publisher }

func New(hub Publisher) Notifier { return Notifier{hub: hub} }

// message is the JSON shape of a message on the stream (same as the REST API).
type message struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	AuthorID       string     `json:"author_id"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
	EditedAt       *time.Time `json:"edited_at"`
	Deleted        bool       `json:"deleted"`
}

func toMessage(m domain.Message) message {
	d := message{ID: string(m.ID), ConversationID: string(m.Conversation), AuthorID: string(m.Author), Body: m.Body,
		CreatedAt: m.CreatedAt, Deleted: m.Deleted}
	if !m.EditedAt.IsZero() {
		d.EditedAt = &m.EditedAt
	}
	return d
}

func (n Notifier) Notify(ctx context.Context, to []domain.UserID, e domain.Event) {
	var data any
	switch e := e.(type) {
	case domain.MessagePosted:
		data = toMessage(e.Message)
	case domain.MessageEdited:
		data = toMessage(e.Message)
	case domain.MessageDeleted:
		data = toMessage(e.Message)
	case domain.Typing:
		data = map[string]string{"conversation_id": string(e.Conversation), "user_id": string(e.User)}
	default:
		return
	}
	users := make([]string, len(to))
	for i, u := range to {
		users[i] = string(u)
	}
	_ = n.hub.Publish(tenant.Workspace(ctx), users, e.EventName(), data)
}
