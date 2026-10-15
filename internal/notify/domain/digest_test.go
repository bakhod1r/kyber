package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// With the daily digest on, email waits for the digest; Telegram stays immediate.
func TestDigestHoldsImmediateEmail(t *testing.T) {
	p := domain.DefaultPrefs()
	if !p.Immediate(domain.ChannelEmail) || !p.Immediate(domain.ChannelTelegram) {
		t.Fatal("defaults are immediate")
	}
	p.EmailDigest = true
	if p.Immediate(domain.ChannelEmail) || !p.Immediate(domain.ChannelTelegram) {
		t.Fatal("digest holds email only")
	}
	p.Email = false
	if p.WantsDigest() {
		t.Fatal("no email at all means no digest")
	}
}

func TestDigestDue(t *testing.T) {
	tashkent := "Asia/Tashkent" // UTC+5
	cases := []struct {
		name string
		now  time.Time
		tz   string
		last time.Time
		due  bool
		day  string
	}{
		{"before 08:00 local", time.Date(2026, 10, 15, 2, 59, 0, 0, time.UTC), tashkent, time.Time{}, false, ""},
		{"at 08:00 local, never sent", time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC), tashkent, time.Time{}, true, "2026-10-15"},
		{"already sent today", time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC), tashkent, time.Date(2026, 10, 15, 3, 1, 0, 0, time.UTC), false, ""},
		{"sent yesterday", time.Date(2026, 10, 15, 3, 30, 0, 0, time.UTC), tashkent, time.Date(2026, 10, 14, 3, 1, 0, 0, time.UTC), true, "2026-10-15"},
		{"UTC when no zone", time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC), "", time.Time{}, true, "2026-10-15"},
		{"unknown zone falls back to UTC", time.Date(2026, 10, 15, 7, 0, 0, 0, time.UTC), "Mars/Base", time.Time{}, false, ""},
	}
	for _, c := range cases {
		day, due := domain.DigestDue(c.now, c.tz, c.last)
		if due != c.due || (due && day != c.day) {
			t.Errorf("%s: DigestDue = %q %v, want %q %v", c.name, day, due, c.day, c.due)
		}
	}
}

func TestRenderDigest(t *testing.T) {
	notes := []domain.Notification{
		{Kind: domain.KindAssigned, IssueKey: "KYB-1", IssueTitle: "Login", ActorName: "Aziza"},
		{Kind: domain.KindCommented, IssueKey: "KYB-2", IssueTitle: "Board", ActorName: "Timur", Excerpt: "Looks good"},
	}
	link := func(n domain.Notification) string { return "https://kyber.test/x/" + n.IssueKey }
	subject, text := domain.RenderDigest(notes, link, "https://kyber.test")
	if subject != "Kyber: 2 unread notifications" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{"Aziza assigned you", "https://kyber.test/x/KYB-1", "Timur commented", "https://kyber.test/x/KYB-2", "https://kyber.test/settings"} {
		if !strings.Contains(text, want) {
			t.Errorf("digest lacks %q:\n%s", want, text)
		}
	}
	if s, _ := domain.RenderDigest(notes[:1], link, "https://kyber.test"); s != "Kyber: 1 unread notification" {
		t.Fatalf("singular subject = %q", s)
	}
}
