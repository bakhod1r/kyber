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
	ThreadID       *string    `json:"thread_id"`
	ReplyCount     int        `json:"reply_count"`
	Kind           string     `json:"kind"`
	File           *fileDTO   `json:"file"`
	PinnedBy       *string    `json:"pinned_by"`
	PinnedAt       *time.Time `json:"pinned_at"`
	Reactions      []reaction `json:"reactions"`
}

type fileDTO struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type reaction struct {
	Emoji   string   `json:"emoji"`
	UserIDs []string `json:"user_ids"`
}

func toMessage(m domain.Message) message {
	d := message{ID: string(m.ID), ConversationID: string(m.Conversation), AuthorID: string(m.Author), Body: m.Body,
		CreatedAt: m.CreatedAt, Deleted: m.Deleted, ReplyCount: m.ReplyCount, Kind: string(m.Kind), Reactions: []reaction{}}
	if !m.EditedAt.IsZero() {
		d.EditedAt = &m.EditedAt
	}
	if m.Thread != "" {
		t := string(m.Thread)
		d.ThreadID = &t
	}
	if f := m.File; f != nil {
		d.File = &fileDTO{Name: f.Name, Type: f.Type, Size: f.Size, URL: "/api/v1/messages/" + string(m.ID) + "/file"}
	}
	if m.PinnedBy != "" {
		by, at := string(m.PinnedBy), m.PinnedAt
		d.PinnedBy, d.PinnedAt = &by, &at
	}
	for _, r := range m.Reactions {
		users := make([]string, len(r.Users))
		for i, u := range r.Users {
			users[i] = string(u)
		}
		d.Reactions = append(d.Reactions, reaction{Emoji: r.Emoji, UserIDs: users})
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
	case domain.MessageReacted:
		data = toMessage(e.Message)
	case domain.MessagePinned:
		data = toMessage(e.Message)
	case domain.Read:
		data = map[string]any{"conversation_id": string(e.Conversation), "user_id": string(e.User), "at": e.At}
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
