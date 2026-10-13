// Package acl is the Whiteboard context's anti-corruption layer to the Chat context.
package acl

import (
	"context"
	"errors"

	chatapp "github.com/bakhod1r/kyber/internal/chat/application"
	chatdomain "github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/whiteboard/application"
)

type Chat interface {
	Conversation(ctx context.Context, actor, id string) (chatapp.View, error)
}

type Conversations struct{ chat Chat }

func NewConversations(chat Chat) Conversations { return Conversations{chat: chat} }

// Members: only members of a conversation open and draw its whiteboards.
func (c Conversations) Members(ctx context.Context, actor, conversation string) ([]string, error) {
	v, err := c.chat.Conversation(ctx, actor, conversation)
	if errors.Is(err, chatdomain.ErrConversationNotFound) || (err == nil && !v.IsMember(chatdomain.UserID(actor))) {
		return nil, application.ErrNoConversation
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, len(v.Members))
	for i, m := range v.Members {
		out[i] = string(m)
	}
	return out, nil
}
