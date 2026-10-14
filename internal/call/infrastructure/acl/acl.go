// Package acl is the Call context's anti-corruption layer to the Chat context.
package acl

import (
	"context"
	"errors"
	"io"

	"github.com/bakhod1r/kyber/internal/call/application"
	chatapp "github.com/bakhod1r/kyber/internal/chat/application"
	chatdomain "github.com/bakhod1r/kyber/internal/chat/domain"
)

type Chat interface {
	Conversation(ctx context.Context, actor, id string) (chatapp.View, error)
	UploadRecording(ctx context.Context, actor, conv, name string, body io.Reader) (*chatdomain.Message, error)
}

// Recordings posts the media server's recordings to the conversation (S104), as the one who recorded.
type Recordings struct{ chat Chat }

func NewRecordings(chat Chat) Recordings { return Recordings{chat: chat} }

func (r Recordings) Post(ctx context.Context, by, conversation, name string, body io.Reader) error {
	_, err := r.chat.UploadRecording(ctx, by, conversation, name, body)
	return err
}

type Conversations struct{ chat Chat }

func NewConversations(chat Chat) Conversations { return Conversations{chat: chat} }

// Members: only members of a conversation take part in its call.
func (c Conversations) Members(ctx context.Context, actor, conversation string) ([]string, error) {
	v, err := c.chat.Conversation(ctx, actor, conversation)
	if errors.Is(err, chatdomain.ErrConversationNotFound) {
		return nil, application.ErrNoConversation
	}
	if err != nil {
		return nil, err
	}
	if !v.IsMember(chatdomain.UserID(actor)) {
		return nil, application.ErrNotMember
	}
	out := make([]string, len(v.Members))
	for i, m := range v.Members {
		out[i] = string(m)
	}
	return out, nil
}
