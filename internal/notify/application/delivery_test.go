package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type contacts struct{ chats map[string]string }

func (c contacts) Email(_ context.Context, user string) (string, error) {
	if user == "ghost" {
		return "", errors.New("user gone")
	}
	return user + "@x.uz", nil
}
func (c contacts) TelegramChat(_ context.Context, user string) (string, bool, error) {
	id, ok := c.chats[user]
	return id, ok, nil
}

type sender struct {
	sent []domain.Delivery
	fail map[string]error // recipient → error
}

func (s *sender) Send(_ context.Context, d domain.Delivery) error {
	if err := s.fail[d.To]; err != nil {
		return err
	}
	s.sent = append(s.sent, d)
	return nil
}

var clock = time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

func deliverySetup(channels ...domain.Channel) (*application.Service, *memory.DeliveryRepository, *memory.PrefsRepository) {
	f := &fakes{
		issues:  map[string]application.IssueInfo{"KYB-1": {Project: "KYB", Title: "Login page", Reporter: "alice", Assignee: "bob"}},
		members: map[string]bool{"KYB/alice": true, "KYB/bob": true},
		names:   map[string]string{"alice": "Alice", "bob": "Bob"},
		emails:  map[string]string{"bob@x.uz": "bob"},
	}
	deliveries, prefs := memory.NewDeliveryRepository(), memory.NewPrefsRepository()
	n := 0
	return application.NewService(application.Deps{
		Repo: memory.NewRepository(), Issues: f, Members: f, Workspaces: f, Users: f, Audience: f,
		Deliveries: deliveries, Prefs: prefs, Contacts: contacts{chats: map[string]string{"bob": "777"}},
		Channels: channels, PublicURL: "https://kyber.example",
		NewID: func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) },
		Now:   func() time.Time { return clock },
	}), deliveries, prefs
}

func TestNotificationsGoToEmailAndTelegram(t *testing.T) {
	ctx := context.Background()
	s, deliveries, _ := deliverySetup(domain.ChannelEmail, domain.ChannelTelegram)
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`))
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`)) // redelivery
	all := deliveries.All()
	if len(all) != 2 {
		t.Fatalf("deliveries = %+v", all)
	}
	email, tg := all[0], all[1]
	if email.Channel != domain.ChannelEmail || email.To != "bob@x.uz" || email.Subject != "[KYB-1] Alice assigned you: Login page" ||
		!strings.HasSuffix(email.Body, "https://kyber.example/projects/KYB?issue=KYB-1") {
		t.Fatalf("email = %+v", email)
	}
	if tg.Channel != domain.ChannelTelegram || tg.To != "777" || tg.Body != email.Body {
		t.Fatalf("telegram = %+v", tg)
	}
}

func TestChannelsRespectConfigReachabilityAndPrefs(t *testing.T) {
	ctx := context.Background()
	// Only email configured on the server: no Telegram deliveries even for linked users.
	s, deliveries, prefs := deliverySetup(domain.ChannelEmail)
	_ = prefs.SetPrefs(ctx, "bob", domain.Prefs{Email: true, Telegram: true})
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	if all := deliveries.All(); len(all) != 1 || all[0].Channel != domain.ChannelEmail {
		t.Fatalf("email-only server = %+v", all)
	}
	// Telegram configured, but alice never connected Telegram; and bob opted out of email.
	s, deliveries, prefs = deliverySetup(domain.ChannelEmail, domain.ChannelTelegram)
	_ = prefs.SetPrefs(ctx, "bob", domain.Prefs{Email: false, Telegram: true})
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	_ = s.OnIssueAssigned(ctx, msg(2, "issue.assigned", `{"key":"KYB-1","to":"alice","by":"bob"}`))
	var got []string
	for _, d := range deliveries.All() {
		got = append(got, string(d.Channel)+":"+d.To)
	}
	if strings.Join(got, ",") != "telegram:777,email:alice@x.uz" {
		t.Fatalf("deliveries = %v", got)
	}
	// No channels configured: in-app only.
	s, deliveries, _ = deliverySetup()
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	if len(deliveries.All()) != 0 {
		t.Fatal("no channels configured, nothing to send")
	}
}

func TestDispatch(t *testing.T) {
	ctx := context.Background()
	s, deliveries, _ := deliverySetup(domain.ChannelEmail, domain.ChannelTelegram)
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	mail := &sender{fail: map[string]error{"bob@x.uz": errors.New("451 try later")}}
	tg := &sender{}
	senders := map[domain.Channel]application.Sender{domain.ChannelEmail: mail, domain.ChannelTelegram: tg}

	n, err := s.Dispatch(ctx, senders)
	if err != nil || n != 1 || len(tg.sent) != 1 {
		t.Fatalf("first run sent %d (%v), telegram %d", n, err, len(tg.sent))
	}
	all := deliveries.All()
	if all[0].Status != domain.DeliveryPending || all[0].Attempts != 1 || all[0].LastError != "451 try later" || all[1].Status != domain.DeliverySent {
		t.Fatalf("after first run = %+v", all)
	}
	// Not due yet; then due after the backoff and the mail server recovered.
	if n, _ := s.Dispatch(ctx, senders); n != 0 {
		t.Fatalf("retried before backoff: %d", n)
	}
	clock = clock.Add(time.Minute)
	defer func() { clock = clock.Add(-time.Minute) }()
	mail.fail = nil
	if n, _ := s.Dispatch(ctx, senders); n != 1 || deliveries.All()[0].Status != domain.DeliverySent {
		t.Fatalf("retry = %d, %+v", n, deliveries.All()[0])
	}
}

func TestDispatchPermanentFailureAndMissingSender(t *testing.T) {
	ctx := context.Background()
	s, deliveries, _ := deliverySetup(domain.ChannelEmail, domain.ChannelTelegram)
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	tg := &sender{fail: map[string]error{"777": application.Permanent(errors.New("bot was blocked by the user"))}}
	_, _ = s.Dispatch(ctx, map[domain.Channel]application.Sender{domain.ChannelTelegram: tg})
	all := deliveries.All()
	if all[1].Status != domain.DeliveryDead || all[1].Attempts != 1 {
		t.Fatalf("blocked bot = %+v", all[1])
	}
	if all[0].Status != domain.DeliveryPending || all[0].LastError != "no sender for channel email" {
		t.Fatalf("missing sender = %+v", all[0])
	}
}

type brokenQueue struct{ claimErr, saveErr, enqueueErr error }

func (b brokenQueue) Enqueue(context.Context, domain.Delivery) error { return b.enqueueErr }
func (b brokenQueue) Claim(context.Context, time.Time, time.Duration, int) ([]domain.Delivery, error) {
	d, _ := domain.NewDelivery("d", "ws", domain.ChannelEmail, "a@x.uz", "s", "b", "k", clock)
	return []domain.Delivery{d}, b.claimErr
}
func (b brokenQueue) Save(context.Context, domain.Delivery) error { return b.saveErr }

func TestDeliveryErrorsSurface(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	if !errors.Is(application.Permanent(boom), boom) || !application.IsPermanent(application.Permanent(boom)) || application.IsPermanent(boom) {
		t.Fatal("Permanent must wrap")
	}
	f := &fakes{issues: map[string]application.IssueInfo{"KYB-1": {Project: "KYB", Title: "t"}}, members: map[string]bool{"KYB/ghost": true, "KYB/bob": true}}
	mk := func(q domain.DeliveryRepository) *application.Service {
		return application.NewService(application.Deps{Repo: memory.NewRepository(), Issues: f, Members: f, Workspaces: f, Users: f, Audience: f,
			Deliveries: q, Prefs: memory.NewPrefsRepository(), Contacts: contacts{}, Channels: []domain.Channel{domain.ChannelEmail},
			NewID: func() string { return "id" }, Now: func() time.Time { return clock }})
	}
	// A contact lookup failure fails the event (the relay retries it).
	if err := mk(memory.NewDeliveryRepository()).OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"ghost","by":"bob"}`)); err == nil {
		t.Fatal("contact error must surface")
	}
	if err := mk(brokenQueue{enqueueErr: boom}).OnIssueAssigned(ctx, msg(2, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"ghost"}`)); !errors.Is(err, boom) {
		t.Fatalf("enqueue err = %v", err)
	}
	if _, err := mk(brokenQueue{claimErr: boom}).Dispatch(ctx, nil); !errors.Is(err, boom) {
		t.Fatalf("claim err = %v", err)
	}
	if _, err := mk(brokenQueue{saveErr: boom}).Dispatch(ctx, map[domain.Channel]application.Sender{domain.ChannelEmail: &sender{}}); !errors.Is(err, boom) {
		t.Fatalf("save err = %v", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s, _, _ := deliverySetup(domain.ChannelEmail, domain.ChannelTelegram)
	got, err := s.Settings(ctx, "bob")
	if err != nil || got.Prefs != domain.DefaultPrefs() || !got.EmailAvailable || !got.TelegramAvailable || !got.TelegramConnected {
		t.Fatalf("bob = %+v, %v", got, err)
	}
	if a, _ := s.Settings(ctx, "alice"); a.TelegramConnected {
		t.Fatal("alice never connected Telegram")
	}
	if err := s.SetPrefs(ctx, "bob", domain.Prefs{Email: false, Telegram: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Settings(ctx, "bob"); got.Prefs.Email || !got.Prefs.Telegram {
		t.Fatalf("saved = %+v", got.Prefs)
	}
	none, _, _ := deliverySetup()
	if got, _ := none.Settings(ctx, "bob"); got.EmailAvailable || got.TelegramAvailable {
		t.Fatalf("unconfigured server = %+v", got)
	}
}

// quiet is a fake Quiet: who is in Do Not Disturb (or a Pomodoro).
type quiet map[string]bool

func (q quiet) Quiet(_ context.Context, user string) (bool, error) { return q[user], nil }

// S66: a chat mention during Do Not Disturb stays in the inbox; no email or Telegram copy.
func TestChatMentionsDuringDNDStayInApp(t *testing.T) {
	for _, dnd := range []bool{false, true} {
		f := &fakes{emails: map[string]string{"bob@x.uz": "bob"}, names: map[string]string{"alice": "Alice", "bob": "Bob"}}
		deliveries := memory.NewDeliveryRepository()
		s := application.NewService(application.Deps{
			Repo: memory.NewRepository(), Issues: f, Members: f, Workspaces: f, Users: f, Audience: f,
			Deliveries: deliveries, Prefs: memory.NewPrefsRepository(), Contacts: contacts{chats: map[string]string{"bob": "777"}},
			Channels: []domain.Channel{domain.ChannelEmail, domain.ChannelTelegram}, PublicURL: "https://kyber.example",
			NewID: func() string { return "00000000-0000-4000-8000-000000000001" }, Now: func() time.Time { return clock },
		}).WithConversations(chats{"c-1": {name: "general", readers: []string{"alice", "bob"}}}).WithQuiet(quiet{"bob": dnd})
		m := msg(1, "chat.message_posted", `{"id":"m-1","conversation_id":"c-1","author_id":"alice","body":"@bob@x.uz look"}`)
		if err := s.OnChatMessage(tenant.With(context.Background(), tenant.Default), m); err != nil {
			t.Fatal(err)
		}
		items, _, _ := s.List(tenant.With(context.Background(), tenant.Default), "bob", false)
		if len(items) != 1 {
			t.Fatalf("dnd=%v inbox = %+v", dnd, items)
		}
		if got, want := len(deliveries.All()), map[bool]int{false: 2, true: 0}[dnd]; got != want {
			t.Fatalf("dnd=%v deliveries = %d, want %d", dnd, got, want)
		}
	}
}
