package application_test

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Audience: project members come from the members map ("project/user"); the workspace is
// everyone in any project. "BROKEN" fails.
func (f *fakes) ProjectMembers(_ context.Context, project string) ([]string, error) {
	if project == "BROKEN" {
		return nil, errors.New("db down")
	}
	var out []string
	for k := range f.members {
		if p, u, _ := strings.Cut(k, "/"); p == project {
			out = append(out, u)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakes) WorkspaceMembers(context.Context) ([]string, error) {
	var out []string
	for k := range f.members {
		_, u, _ := strings.Cut(k, "/")
		out = append(out, u)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// linkGroup connects a Telegram group to the project as actor.
func linkGroup(t *testing.T, s *application.Service, actor, project, chat string) {
	t.Helper()
	link, _, err := s.StartGroupLink(tenant.With(context.Background(), tenant.Default), actor, project)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OnGroupStart(context.Background(), startPayload(t, link), chat, "Devs"); err != nil {
		t.Fatal(err)
	}
}

const announced = `{"id":"a-1","workspace":"00000000-0000-4000-8000-000000000001","project":"KYB","title":"Release Friday","excerpt":"Deploy at 16:00","author":"alice"}`

func TestAnnouncementReachesTheProjectButNotItsAuthor(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnAnnouncement(ctx, msg(5, "announcement.published", announced)); err != nil {
		t.Fatal(err)
	}
	_ = s.OnAnnouncement(ctx, msg(5, "announcement.published", announced)) // redelivery
	if got := inbox(t, s, "bob"); got != "announcement::Alice" {
		t.Fatalf("bob = %q", got)
	}
	if got := inbox(t, s, "alice"); got != "" {
		t.Fatalf("the author is not notified: %q", got)
	}
	items, _, _ := s.List(ctx, "dave", false)
	if len(items) != 1 || items[0].Project != "KYB" || items[0].IssueTitle != "Release Friday" || items[0].Excerpt != "Deploy at 16:00" {
		t.Fatalf("dave = %+v", items)
	}
}

func TestWorkspaceAnnouncementReachesEveryone(t *testing.T) {
	s, _ := setup()
	payload := strings.Replace(announced, `"project":"KYB",`, "", 1)
	if err := s.OnAnnouncement(context.Background(), msg(6, "announcement.published", payload)); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"bob", "carol", "dave"} {
		if got := inbox(t, s, u); got != "announcement::Alice" {
			t.Errorf("%s = %q", u, got)
		}
	}
}

func TestAnnouncementFailures(t *testing.T) {
	s, _ := setup()
	if err := s.OnAnnouncement(context.Background(), msg(7, "announcement.published", `not json`)); err != nil {
		t.Fatalf("malformed events are skipped: %v", err)
	}
	broken := strings.Replace(announced, `"project":"KYB"`, `"project":"BROKEN"`, 1)
	if err := s.OnAnnouncement(context.Background(), msg(8, "announcement.published", broken)); err == nil {
		t.Fatal("audience failure is retried")
	}
}

func TestAnnouncementEmailLinksToTheFeed(t *testing.T) {
	s, deliveries, _ := deliverySetup(domain.ChannelEmail)
	if err := s.OnAnnouncement(context.Background(), msg(9, "announcement.published", announced)); err != nil {
		t.Fatal(err)
	}
	all := deliveries.All()
	if len(all) != 1 || all[0].Subject != "[KYB] Announcement: Release Friday" || !strings.HasSuffix(all[0].Body, "https://kyber.example/projects/KYB/announcements") {
		t.Fatalf("deliveries = %+v", all)
	}
	s, deliveries, _ = deliverySetup(domain.ChannelEmail)
	_ = s.OnAnnouncement(context.Background(), msg(10, "announcement.published", strings.Replace(announced, `"project":"KYB",`, "", 1)))
	if all := deliveries.All(); len(all) != 1 || !strings.HasSuffix(all[0].Body, "https://kyber.example/") {
		t.Fatalf("workspace deliveries = %+v", all)
	}
}

func TestAnnouncementPostsToTheProjectsTelegramGroups(t *testing.T) {
	ctx := context.Background()
	s, _, deliveries, _ := groupSetup(admins{})
	linkGroup(t, s, "alice", "KYB", "-1001")
	if err := s.OnProjectEvent(ctx, msg(11, "announcement.published", announced)); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, d := range deliveries.All() {
		if d.To == "-1001" {
			texts = append(texts, d.Body)
		}
	}
	if len(texts) != 1 || texts[0] != "📣 Alice: Release Friday\nDeploy at 16:00\nhttps://kyber.example/projects/KYB/announcements" {
		t.Fatalf("group posts = %q", texts)
	}
	// Workspace-wide announcements have no project group to post to.
	if err := s.OnProjectEvent(ctx, msg(12, "announcement.published", strings.Replace(announced, `"project":"KYB",`, "", 1))); err != nil {
		t.Fatal(err)
	}
}
