package telegramsend_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/adapter/telegramsend"
	"github.com/bakhod1r/kyber/internal/notify/app"
	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type bot struct {
	chat, text string
	err        error
}

func (b *bot) SendMessage(_ context.Context, chat, text string) error {
	b.chat, b.text = chat, text
	return b.err
}

func TestSend(t *testing.T) {
	d, _ := domain.NewDelivery("d", "ws", domain.ChannelTelegram, "777", "subject", "Aziza assigned you KYB-1", "k", time.Now())
	b := &bot{}
	if err := telegramsend.New(b).Send(context.Background(), d); err != nil || b.chat != "777" || b.text != "Aziza assigned you KYB-1" {
		t.Fatalf("sent %q to %q: %v", b.text, b.chat, err)
	}
	for msg, permanent := range map[string]bool{
		"telegram sendMessage: Forbidden: bot was blocked by the user": true,
		"telegram sendMessage: Bad Request: chat not found":            true,
		"telegram sendMessage: Forbidden: user is deactivated":         true,
		"telegram sendMessage: Too Many Requests: retry after 5":       false,
		"telegram sendMessage: connection reset":                       false,
	} {
		b.err = errors.New(msg)
		err := telegramsend.New(b).Send(context.Background(), d)
		if err == nil || app.IsPermanent(err) != permanent {
			t.Errorf("%q: permanent = %v, want %v", msg, app.IsPermanent(err), permanent)
		}
	}
}
