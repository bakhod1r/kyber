package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// Messenger sends a text to a Telegram chat (the bot).
type Messenger interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

// WithTelegramOTP enables "log in with a code from the Telegram bot" (KYB-S40).
func WithTelegramOTP(store domain.OTPChallenges, bot Messenger) Option {
	return func(s *Service) { s.otps, s.bot = store, bot }
}

const howTo = "To log in, open Kyber and choose “Log in with Telegram code”, then press the link it shows."

func (s *Service) otpEnabled() bool { return s.otps != nil && s.bot != nil && s.external != nil }

// StartTelegramOTP creates a challenge; the browser keeps its ID, the deep link carries its Nonce.
// Starts are throttled per client IP by the login limiter.
func (s *Service) StartTelegramOTP(ctx context.Context, ip string) (*domain.OTPChallenge, error) {
	if !s.otpEnabled() {
		return nil, ErrExternalDisabled
	}
	allowed, retryAfter, err := s.limiter.Allow(ctx, "otp-start:"+ip)
	if err != nil {
		return nil, fmt.Errorf("otp rate limiter: %w", err)
	}
	if !allowed {
		return nil, &TooManyAttemptsError{RetryAfter: retryAfter}
	}
	var raw [24]byte
	_, _ = rand.Read(raw[:])
	c := domain.NewOTPChallenge(s.newID(), base64.RawURLEncoding.EncodeToString(raw[:]), s.clock.Now())
	return c, s.otps.CreateOTP(ctx, c)
}

// OnTelegramStart handles "/start <nonce>" from the bot: bind the Telegram user and send the code.
func (s *Service) OnTelegramStart(ctx context.Context, nonce, telegramID, chatID, name string) error {
	if !s.otpEnabled() {
		return ErrExternalDisabled
	}
	c, err := s.otps.OTPByNonce(ctx, nonce)
	if err == nil {
		code := domain.NewOTPCode()
		err = s.otps.UpdateOTP(ctx, c.ID, func(c *domain.OTPChallenge) error {
			return c.Deliver(telegramID, name, code, s.clock.Now())
		})
		if err == nil {
			return s.bot.SendMessage(ctx, chatID, fmt.Sprintf(
				"Your Kyber login code: %s\n\nIt is valid for 5 minutes. Never share it — Kyber will never ask you for it.", code))
		}
	}
	if errors.Is(err, domain.ErrOTPNotFound) || errors.Is(err, domain.ErrOTPExpired) {
		_ = s.bot.SendMessage(ctx, chatID, "This login link is not valid any more. "+howTo)
	}
	return err
}

// VerifyTelegramOTP checks the code typed on the site and signs the Telegram user in.
func (s *Service) VerifyTelegramOTP(ctx context.Context, id, code string) (string, *domain.User, error) {
	if !s.otpEnabled() {
		return "", nil, ErrExternalDisabled
	}
	var who domain.OTPChallenge
	err := s.otps.UpdateOTP(ctx, id, func(c *domain.OTPChallenge) error {
		who = *c
		return c.Verify(code, s.clock.Now())
	})
	if err != nil {
		return "", nil, err
	}
	return s.LoginExternal(ctx, ExternalProfile{Provider: domain.ProviderTelegram, Subject: who.TelegramID, Name: who.Name})
}
