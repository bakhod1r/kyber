package domain_test

import (
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

func TestRenderAnnouncement(t *testing.T) {
	n := domain.Notification{Kind: domain.KindAnnouncement, Project: "KYB", IssueTitle: "Release Friday", ActorName: "Aziza", Excerpt: "Deploy at 16:00"}
	subject, text := domain.Render(n, "https://kyber.example/projects/KYB/announcements")
	if subject != "[KYB] Announcement: Release Friday" || !strings.HasPrefix(text, "📣 Aziza announced «Release Friday»:\n\nDeploy at 16:00") ||
		!strings.HasSuffix(text, "/announcements") {
		t.Fatalf("%q / %q", subject, text)
	}
	n.Project = ""
	if subject, _ := domain.Render(n, "https://kyber.example/"); subject != "Announcement: Release Friday" {
		t.Fatalf("workspace subject = %q", subject)
	}
	n.Excerpt = ""
	if _, text := domain.Render(n, "L"); text != "📣 Aziza announced «Release Friday».\n\nL" {
		t.Fatalf("no text = %q", text)
	}
}

func TestAnnouncementRecipients(t *testing.T) {
	got := domain.AnnouncementRecipients("u-1", []domain.UserID{"u-1", "u-2", "u-3", "u-2", ""})
	if len(got) != 2 || got[0] != "u-2" || got[1] != "u-3" {
		t.Fatalf("recipients = %v", got)
	}
}

func TestAnnouncementGroupMessage(t *testing.T) {
	e := domain.ProjectEvent{Kind: domain.EventAnnouncement, Actor: "Aziza", Title: "Release Friday", Text: "Deploy at 16:00"}
	if got := e.Message("https://k/projects/KYB/announcements"); got != "📣 Aziza: Release Friday\nDeploy at 16:00\nhttps://k/projects/KYB/announcements" {
		t.Fatalf("message = %q", got)
	}
	e.Text = ""
	if got := e.Message("L"); got != "📣 Aziza: Release Friday\nL" {
		t.Fatalf("message = %q", got)
	}
}
