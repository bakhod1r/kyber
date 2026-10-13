// Package telegramsend delivers notifications as Telegram messages through the Kyber bot.
package telegramsend

import (
	"context"
	"strings"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// Messenger is the bot client (identity/adapter/telegram.Bot satisfies it).
type Messenger interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

type Sender struct{ bot Messenger }

func New(b Messenger) Sender { return Sender{bot: b} }

// permanent are Telegram answers retrying cannot fix: the user blocked the bot, or the chat is gone.
var permanent = []string{"Forbidden:", "chat not found", "user is deactivated"}

func (s Sender) Send(ctx context.Context, d domain.Delivery) error {
	err := s.bot.SendMessage(ctx, d.To, d.Body)
	if err == nil {
		return nil
	}
	for _, p := range permanent {
		if strings.Contains(err.Error(), p) {
			return application.Permanent(err)
		}
	}
	return err
}
