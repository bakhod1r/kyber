package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var ErrTelegramTaken = errors.New("this Telegram account is connected to another Kyber account")

// WithTelegramConnect lets signed-in users connect their Telegram chat for notifications through a
// deep link to the bot (needs WithTelegramOTP for the bot and WithExternalIdentities).
func WithTelegramConnect(store domain.ConnectTokens, botName string) Option {
	return func(s *Service) { s.connect, s.botName = store, botName }
}

func (s *Service) connectEnabled() bool {
	return s.connect != nil && s.bot != nil && s.external != nil && s.botName != ""
}

// StartTelegramConnect returns the deep link (t.me/<bot>?start=<one-time token>) and its expiry.
func (s *Service) StartTelegramConnect(ctx context.Context, user string) (string, time.Time, error) {
	if !s.connectEnabled() {
		return "", time.Time{}, ErrExternalDisabled
	}
	raw, tok := domain.NewConnectToken(domain.UserID(user), s.clock.Now())
	if err := s.connect.CreateConnect(ctx, tok); err != nil {
		return "", time.Time{}, err
	}
	link := "https://t.me/" + url.PathEscape(s.botName) + "?start=" + url.QueryEscape(raw)
	return link, tok.ExpiresAt, nil
}

// connectTelegram handles "/start c_<token>": it binds the Telegram account to the token's user.
// A Telegram account already bound to someone else is never moved.
func (s *Service) connectTelegram(ctx context.Context, payload, telegramID, chatID string) error {
	if !s.connectEnabled() {
		return ErrExternalDisabled
	}
	user, err := s.connect.ConsumeConnect(ctx, domain.HashConnectToken(payload), s.clock.Now())
	if err != nil {
		if errors.Is(err, domain.ErrConnectTokenInvalid) {
			_ = s.bot.SendMessage(ctx, chatID, "This connect link is not valid any more. In Kyber open "+
				"Notification settings and press “Connect Telegram” to get a new one.")
		}
		return err
	}
	owner, err := s.external.LinkedUser(ctx, domain.ProviderTelegram, telegramID)
	switch {
	case err == nil && owner != user:
		_ = s.bot.SendMessage(ctx, chatID, "This Telegram account is already connected to another Kyber account.")
		return ErrTelegramTaken
	case errors.Is(err, domain.ErrNotLinked):
		if err := s.external.Link(ctx, domain.ExternalIdentity{Provider: domain.ProviderTelegram, Subject: telegramID, UserID: user}); err != nil {
			return err
		}
	case err != nil:
		return err
	}
	u, err := s.users.ByID(ctx, user)
	if err != nil {
		return err
	}
	return s.bot.SendMessage(ctx, chatID, fmt.Sprintf("✅ Telegram connected. Kyber notifications for %s will arrive here. "+
		"You can turn them off in Notification settings.", u.Email()))
}
