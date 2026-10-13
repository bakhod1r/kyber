package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func TestMeetingText(t *testing.T) {
	start := time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		event, want string
	}{
		{"meeting.scheduled", "Sat 17 Oct, 15:00–16:00 UTC"},
		{"meeting.cancelled", "Cancelled — was Sat 17 Oct, 15:00–16:00 UTC"},
		{"meeting.reminder", "Starts at 15:00 UTC"},
		{"meeting.other", ""},
	} {
		if got := domain.MeetingText(tc.event, start, start.Add(time.Hour)); got != tc.want {
			t.Errorf("%s = %q", tc.event, got)
		}
	}
}

func TestRenderMeeting(t *testing.T) {
	n := domain.Notification{Kind: domain.KindMeeting, Project: "KYB", IssueTitle: "Sprint review", ActorName: "Aziza", Excerpt: "Sat 17 Oct, 15:00–16:00 UTC"}
	subject, text := domain.Render(n, "https://k/meetings")
	if subject != "[KYB] Sprint review — Sat 17 Oct, 15:00–16:00 UTC" || !strings.HasPrefix(text, "📅 Aziza: «Sprint review»\nSat 17 Oct") || !strings.HasSuffix(text, "https://k/meetings") {
		t.Fatalf("%q / %q", subject, text)
	}
	n.Project, n.ActorName = "", ""
	if subject, text := domain.Render(n, "L"); subject != "Sprint review — Sat 17 Oct, 15:00–16:00 UTC" || !strings.HasPrefix(text, "📅 «Sprint review»") {
		t.Fatalf("%q / %q", subject, text)
	}
	e := domain.ProjectEvent{Kind: domain.EventMeeting, Title: "Sprint review", Text: "Starts at 15:00 UTC"}
	if got := e.Message("L"); got != "📅 Sprint review — Starts at 15:00 UTC\nL" {
		t.Fatalf("group = %q", got)
	}
}
