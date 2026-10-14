package acl

import (
	"context"
	"errors"

	chatapp "github.com/bakhod1r/kyber/internal/chat/application"
	chatdomain "github.com/bakhod1r/kyber/internal/chat/domain"
)

// Chat is the Chat context's authorized read of a conversation.
type Chat interface {
	Conversation(ctx context.Context, actor, id string) (chatapp.View, error)
}

// Conversations: may someone read a chat channel (S76: a meeting in its huddle).
type Conversations struct{ chat Chat }

func NewConversations(chat Chat) Conversations { return Conversations{chat: chat} }

func (c Conversations) Readable(ctx context.Context, user, conversation string) (bool, error) {
	_, err := c.chat.Conversation(ctx, user, conversation)
	if errors.Is(err, chatdomain.ErrConversationNotFound) {
		return false, nil
	}
	return err == nil, err
}
