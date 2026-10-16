package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/memory"
)

type digestFixture struct {
	s          *application.Service
	deliveries *memory.DeliveryRepository
	prefs      *memory.PrefsRepository
	now        time.Time
}

func newDigestFixture() *digestFixture {
	f := &fakes{
		issues:  map[string]application.IssueInfo{"KYB-1": {Project: "KYB", Title: "Login page", Reporter: "alice", Assignee: "bob"}},
		members: map[string]bool{"KYB/alice": true, "KYB/bob": true, "KYB/abe": true},
		names:   map[string]string{"alice": "Alice", "bob": "Bob", "abe": "Abe"},
	}
	x := &digestFixture{deliveries: memory.NewDeliveryRepository(), prefs: memory.NewPrefsRepository(), now: time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC)}
	n := 0
	x.s = application.NewService(application.Deps{
		Repo: memory.NewRepository(), Issues: f, Members: f, Workspaces: f, Users: f, Audience: f,
		Deliveries: x.deliveries, Prefs: x.prefs, Contacts: contacts{chats: map[string]string{"bob": "777"}, gone: "abe"},
		Channels: []domain.Channel{domain.ChannelEmail, domain.ChannelTelegram}, PublicURL: "https://kyber.example",
		NewID: func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) },
		Now:   func() time.Time { return x.now },
	})
	return x
}

func (x *digestFixture) emails() []domain.Delivery {
	var out []domain.Delivery
	for _, d := range x.deliveries.All() {
		if d.Channel == domain.ChannelEmail {
			out = append(out, d)
		}
	}
	return out
}

// B9: with the daily digest on, notifications are not emailed one by one (Telegram still is); at 08:00 in the
// user's time zone one email lists what is unread, once a day even when several replicas run the job.
func TestDailyDigest(t *testing.T) {
	ctx := context.Background()
	x := newDigestFixture()
	if err := x.prefs.SetPrefs(ctx, "bob", domain.Prefs{Email: true, Telegram: true, TimeZone: "Asia/Tashkent", EmailDigest: true}); err != nil {
		t.Fatal(err)
	}
	_ = x.s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`))
	if got := x.emails(); len(got) != 0 {
		t.Fatalf("immediate email despite the digest: %+v", got)
	}
	if all := x.deliveries.All(); len(all) != 1 || all[0].Channel != domain.ChannelTelegram {
		t.Fatalf("Telegram stays immediate: %+v", all)
	}

	// 07:00 in Tashkent: not yet.
	if n, err := x.s.SendDigests(ctx); err != nil || n != 0 {
		t.Fatalf("early SendDigests = %d, %v", n, err)
	}
	// 08:00 in Tashkent (03:00 UTC): one digest, also when a second replica runs the job at the same time.
	x.now = time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC)
	if n, err := x.s.SendDigests(ctx); err != nil || n != 1 {
		t.Fatalf("SendDigests = %d, %v", n, err)
	}
	_, _ = x.s.SendDigests(ctx)
	got := x.emails()
	if len(got) != 1 || got[0].To != "bob@x.uz" || got[0].Subject != "Kyber: 1 unread notification" ||
		!strings.Contains(got[0].Body, "Alice assigned you") || !strings.Contains(got[0].Body, "https://kyber.example/projects/KYB?issue=KYB-1") {
		t.Fatalf("digest = %+v", got)
	}

	// The next day only what arrived since the last digest is listed; nothing new means no email.
	x.now = x.now.Add(24 * time.Hour)
	if n, _ := x.s.SendDigests(ctx); n != 0 || len(x.emails()) != 1 {
		t.Fatalf("empty digest sent: %d", n)
	}
}

func TestDigestOffEmailsRightAway(t *testing.T) {
	ctx := context.Background()
	x := newDigestFixture()
	_ = x.s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`))
	if len(x.emails()) != 1 {
		t.Fatalf("emails = %+v", x.emails())
	}
	x.now = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	if n, _ := x.s.SendDigests(ctx); n != 0 {
		t.Fatalf("digest for a user without it: %d", n)
	}
}

// A digest run with nothing unread does not use up the day: what arrives later that day still goes out.
func TestEmptyDigestRunDoesNotUseUpTheDay(t *testing.T) {
	ctx := context.Background()
	x := newDigestFixture()
	_ = x.prefs.SetPrefs(ctx, "bob", domain.Prefs{Email: true, Telegram: true, EmailDigest: true})
	x.now = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	if n, _ := x.s.SendDigests(ctx); n != 0 {
		t.Fatalf("digest with nothing unread: %d", n)
	}
	x.now = x.now.Add(time.Minute)
	_ = x.s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`))
	x.now = x.now.Add(time.Minute)
	if n, err := x.s.SendDigests(ctx); err != nil || n != 1 {
		t.Fatalf("later SendDigests = %d, %v", n, err)
	}
}

// One user's failure (a removed account) does not hold back everyone else's digest.
func TestDigestFailureOfOneUserSparesOthers(t *testing.T) {
	ctx := context.Background()
	x := newDigestFixture()
	on := domain.Prefs{Email: true, Telegram: true, EmailDigest: true}
	_ = x.prefs.SetPrefs(ctx, "bob", on)
	_ = x.prefs.SetPrefs(ctx, "abe", on) // processed first; contacts cannot find this one
	x.now = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	_ = x.s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`))
	_ = x.s.OnIssueAssigned(ctx, msg(2, "issue.assigned", `{"key":"KYB-1","from":"bob","to":"abe","by":"alice"}`))
	n, err := x.s.SendDigests(ctx)
	if err == nil || n != 1 || len(x.emails()) != 1 || x.emails()[0].To != "bob@x.uz" {
		t.Fatalf("SendDigests = %d, %v; emails %+v", n, err, x.emails())
	}
}
