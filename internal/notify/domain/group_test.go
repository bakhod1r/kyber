package domain_test

import (
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func TestGroupToken(t *testing.T) {
	raw, tok := domain.NewGroupToken("ws", "KYB", "u-lead", t0)
	if !strings.HasPrefix(raw, "g_") || len(raw) > 64 || strings.ContainsAny(raw, "+/=") {
		t.Fatalf("raw %q is not a valid startgroup payload", raw)
	}
	if tok.Hash != domain.HashGroupToken(raw) || tok.Hash == raw || tok.Workspace != "ws" || tok.Project != "KYB" ||
		tok.User != "u-lead" || !tok.ExpiresAt.Equal(t0.Add(domain.GroupTokenTTL)) {
		t.Fatalf("token = %+v", tok)
	}
	if again, _ := domain.NewGroupToken("ws", "KYB", "u-lead", t0); again == raw {
		t.Fatal("tokens must be random")
	}
}

func TestProjectEventMessages(t *testing.T) {
	cases := []struct {
		e    domain.ProjectEvent
		want string
	}{
		{domain.ProjectEvent{Kind: domain.EventIssueCreated, Actor: "Aziza", IssueKey: "KYB-3", Title: "Login"}, "🆕 Aziza created KYB-3 «Login»"},
		{domain.ProjectEvent{Kind: domain.EventStatusChanged, Actor: "Timur", IssueKey: "KYB-3", Title: "Login", From: "In Progress", To: "Done"}, "🔄 Timur moved KYB-3 «Login»: In Progress → Done"},
		{domain.ProjectEvent{Kind: domain.EventSprintStarted, Sprint: "Sprint 12"}, "🏁 Sprint 12 started"},
		{domain.ProjectEvent{Kind: domain.EventSprintCompleted, Sprint: "Sprint 12"}, "✅ Sprint 12 completed"},
	}
	for _, c := range cases {
		got := c.e.Message("https://kyber.example/projects/KYB")
		if !strings.HasPrefix(got, c.want) || !strings.HasSuffix(got, "https://kyber.example/projects/KYB") {
			t.Errorf("%s: %q", c.e.Kind, got)
		}
	}
}
