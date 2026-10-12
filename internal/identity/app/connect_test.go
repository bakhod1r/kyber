package app_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func setupConnect(t *testing.T) (*app.Service, *memory.Repository, *clock, *inbox, *domain.User) {
	t.Helper()
	repo := memory.NewRepository()
	c := &clock{now: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	box := &inbox{}
	s := app.NewService(repo, repo, &plainHasher{}, c, func() string { return "40000000-0000-4000-8000-000000000001" },
		app.WithExternalIdentities(repo), app.WithTelegramOTP(repo, box), app.WithTelegramConnect(repo, "KyberBot"))
	u, err := s.Signup(context.Background(), app.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	if err != nil {
		t.Fatal(err)
	}
	return s, repo, c, box, u
}

// payload extracts ?start=… from the deep link.
func payload(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Host != "t.me" || u.Path != "/KyberBot" {
		t.Fatalf("link = %q", link)
	}
	return u.Query().Get("start")
}

func TestConnectTelegram(t *testing.T) {
	ctx := context.Background()
	s, repo, clk, box, u := setupConnect(t)
	link, expires, err := s.StartTelegramConnect(ctx, string(u.ID()))
	if err != nil || !expires.Equal(clk.now.Add(domain.ConnectTTL)) {
		t.Fatalf("start = %q %v %v", link, expires, err)
	}
	p := payload(t, link)
	if err := s.OnTelegramStart(ctx, p, "777", "777", "Ali T"); err != nil {
		t.Fatal(err)
	}
	if chat, err := repo.SubjectOf(ctx, u.ID(), domain.ProviderTelegram); err != nil || chat != "777" {
		t.Fatalf("linked chat = %q, %v", chat, err)
	}
	if len(box.text) != 1 || !strings.Contains(box.text[0], "connected") || !strings.Contains(box.text[0], "ali@x.uz") {
		t.Fatalf("reply = %v", box.text)
	}
	// The link works once.
	if err := s.OnTelegramStart(ctx, p, "777", "777", "Ali T"); !errors.Is(err, domain.ErrConnectTokenInvalid) {
		t.Fatalf("reuse err = %v", err)
	}
	if !strings.Contains(box.text[1], "not valid any more") {
		t.Fatalf("reuse reply = %q", box.text[1])
	}
	// Connecting the same Telegram again from a fresh link is harmless.
	link, _, _ = s.StartTelegramConnect(ctx, string(u.ID()))
	if err := s.OnTelegramStart(ctx, payload(t, link), "777", "777", "Ali T"); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
}

func TestConnectTelegramOwnedByAnotherUser(t *testing.T) {
	ctx := context.Background()
	s, repo, _, box, u := setupConnect(t)
	_ = repo.Link(ctx, domain.ExternalIdentity{Provider: domain.ProviderTelegram, Subject: "999", UserID: "someone-else"})
	link, _, _ := s.StartTelegramConnect(ctx, string(u.ID()))
	if err := s.OnTelegramStart(ctx, payload(t, link), "999", "999", "Eve"); !errors.Is(err, app.ErrTelegramTaken) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(box.text[0], "another Kyber account") {
		t.Fatalf("reply = %v", box.text)
	}
	if _, err := repo.SubjectOf(ctx, u.ID(), domain.ProviderTelegram); !errors.Is(err, domain.ErrNotLinked) {
		t.Fatal("must not be linked")
	}
}

func TestConnectTelegramExpiresAndNeedsTheBot(t *testing.T) {
	ctx := context.Background()
	s, _, clk, _, u := setupConnect(t)
	link, _, _ := s.StartTelegramConnect(ctx, string(u.ID()))
	clk.now = clk.now.Add(domain.ConnectTTL + time.Second)
	if err := s.OnTelegramStart(ctx, payload(t, link), "777", "777", "Ali"); !errors.Is(err, domain.ErrConnectTokenInvalid) {
		t.Fatalf("expired err = %v", err)
	}
	repo := memory.NewRepository()
	off := app.NewService(repo, repo, &plainHasher{}, clk, func() string { return "x" }, app.WithExternalIdentities(repo))
	if _, _, err := off.StartTelegramConnect(ctx, "u"); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("no bot err = %v", err)
	}
	if err := off.OnTelegramStart(ctx, "c_whatever", "1", "1", "x"); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("no bot start err = %v", err)
	}
}
