package domain_test

import (
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func TestPrefs(t *testing.T) {
	p := domain.DefaultPrefs()
	if !p.Wants(domain.ChannelEmail) || !p.Wants(domain.ChannelTelegram) || p.Wants("fax") {
		t.Fatal("defaults")
	}
	p.Telegram = false
	if p.Wants(domain.ChannelTelegram) || !p.Wants(domain.ChannelEmail) {
		t.Fatal("opt out")
	}
}

func TestRender(t *testing.T) {
	n := domain.Notification{IssueKey: "KYB-12", IssueTitle: "Login", ActorName: "Aziza", Excerpt: "Looks good"}
	cases := map[domain.Kind][2]string{
		domain.KindAssigned:     {"[KYB-12] Aziza assigned you: Login", "Aziza assigned you KYB-12 «Login»."},
		domain.KindMentioned:    {"[KYB-12] Aziza mentioned you: Login", "Looks good"},
		domain.KindCommented:    {"[KYB-12] Aziza commented: Login", "Looks good"},
		domain.KindTransitioned: {"[KYB-12] Aziza changed the status: Login", "Aziza moved KYB-12 «Login»: Looks good."},
	}
	for kind, want := range cases {
		n.Kind = kind
		subject, text := domain.Render(n, "https://kyber.example/projects/KYB?issue=KYB-12")
		if subject != want[0] || !strings.Contains(text, want[1]) || !strings.HasSuffix(text, "?issue=KYB-12") {
			t.Errorf("%s: %q / %q", kind, subject, text)
		}
	}
}
