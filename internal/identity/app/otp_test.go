package app_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

type inbox struct {
	chat, text []string
	fail       error
}

func (b *inbox) SendMessage(_ context.Context, chat, text string) error {
	if b.fail != nil {
		return b.fail
	}
	b.chat, b.text = append(b.chat, chat), append(b.text, text)
	return nil
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

func setupOTP() (*app.Service, *clock, *inbox) {
	repo := memory.NewRepository()
	n := 0
	ids := func() string { n++; return fmt.Sprintf("c-%d", n) }
	c := &clock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	box := &inbox{}
	s := app.NewService(repo, repo, &plainHasher{}, c, ids, app.WithExternalIdentities(repo), app.WithTelegramOTP(repo, box))
	return s, c, box
}

func TestTelegramOTP(t *testing.T) {
	ctx := context.Background()
	s, clk, box := setupOTP()

	// AC1 the site starts a challenge; the deep-link nonce differs from the browser's id.
	ch, err := s.StartTelegramOTP(ctx, "1.1.1.1")
	if err != nil || ch.ID != "c-1" || len(ch.Nonce) < 20 || ch.Nonce == ch.ID {
		t.Fatalf("start = %+v, %v", ch, err)
	}
	// AC2 pressing Start in the bot sends a 6-digit code to that chat.
	if err := s.OnTelegramStart(ctx, ch.Nonce, "42", "4200", "Bobur"); err != nil {
		t.Fatal(err)
	}
	if len(box.chat) != 1 || box.chat[0] != "4200" || !strings.Contains(box.text[0], "Never share") {
		t.Fatalf("sent = %v %v", box.chat, box.text)
	}
	code := codeRe.FindString(box.text[0])

	// AC3 a wrong code is refused, the right one signs in as the Telegram user.
	if _, _, err := s.VerifyTelegramOTP(ctx, ch.ID, "000000"); !errors.Is(err, domain.ErrOTPWrongCode) && code != "000000" {
		t.Fatalf("wrong code err = %v", err)
	}
	clk.now = clk.now.Add(time.Minute)
	token, u, err := s.VerifyTelegramOTP(ctx, ch.ID, code)
	if err != nil || token == "" || u.Name() != "Bobur" || u.Email() != "telegram-42@users.kyber.invalid" {
		t.Fatalf("verify = %q %+v %v", token, u, err)
	}
	// The same Telegram account maps to the same user as the Login Widget.
	_, w, _ := s.LoginExternal(ctx, app.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "42", Name: "Bobur"})
	if w.ID() != u.ID() {
		t.Fatalf("widget user %s != otp user %s", w.ID(), u.ID())
	}
	// AC4 single use.
	if _, _, err := s.VerifyTelegramOTP(ctx, ch.ID, code); !errors.Is(err, domain.ErrOTPUsed) {
		t.Fatalf("replay err = %v", err)
	}
}

func TestTelegramOTPFailures(t *testing.T) {
	ctx := context.Background()
	s, clk, box := setupOTP()
	if err := s.OnTelegramStart(ctx, "unknown", "42", "4200", "B"); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("unknown nonce err = %v", err)
	}
	if err := s.OnTelegramStart(ctx, "", "42", "4200", "B"); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("bare /start err = %v", err)
	}
	if len(box.text) != 2 || !strings.Contains(box.text[1], "Log in with Telegram") {
		t.Fatalf("a bare or stale /start gets instructions, got %v", box.text)
	}
	if _, _, err := s.VerifyTelegramOTP(ctx, "missing", "123456"); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	ch, _ := s.StartTelegramOTP(ctx, "1.1.1.1")
	clk.now = clk.now.Add(domain.OTPTTL + time.Second)
	if err := s.OnTelegramStart(ctx, ch.Nonce, "42", "4200", "B"); !errors.Is(err, domain.ErrOTPExpired) {
		t.Fatalf("expired start err = %v", err)
	}
	box.fail = errors.New("telegram down")
	ch2, _ := s.StartTelegramOTP(ctx, "1.1.1.1")
	if err := s.OnTelegramStart(ctx, ch2.Nonce, "42", "4200", "B"); err == nil {
		t.Fatal("send failure must surface")
	}
	off, _, _ := setup()
	if _, err := off.StartTelegramOTP(ctx, "1.1.1.1"); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("disabled err = %v", err)
	}
	if err := off.OnTelegramStart(ctx, "n", "1", "1", "x"); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("disabled err = %v", err)
	}
	if _, _, err := off.VerifyTelegramOTP(ctx, "c", "1"); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("disabled err = %v", err)
	}
}

func TestTelegramOTPStartIsThrottled(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setupOTP()
	var err error
	for range app.LoginAttempts + 1 {
		_, err = s.StartTelegramOTP(ctx, "6.6.6.6")
	}
	var tooMany *app.TooManyAttemptsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.StartTelegramOTP(ctx, "7.7.7.7"); err != nil {
		t.Fatalf("other IPs are unaffected: %v", err)
	}
}

type brokenLimiter struct{}

func (brokenLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return false, 0, errors.New("redis down")
}

func TestTelegramOTPStartLimiterDown(t *testing.T) {
	repo := memory.NewRepository()
	s := app.NewService(repo, repo, &plainHasher{}, &clock{}, func() string { return "c-1" },
		app.WithExternalIdentities(repo), app.WithTelegramOTP(repo, &inbox{}), app.WithLoginLimiter(brokenLimiter{}))
	if _, err := s.StartTelegramOTP(context.Background(), "1.1.1.1"); err == nil {
		t.Fatal("limiter outage must fail closed")
	}
}
