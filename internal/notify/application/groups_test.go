package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type admins map[string]error // user → Administer result (missing = allowed)

func (a admins) Administer(_ context.Context, actor, project string) error { return a[actor] }

type replies struct{ sent []string }

func (r *replies) SendMessage(_ context.Context, chat, text string) error {
	r.sent = append(r.sent, chat+": "+text)
	return nil
}

type lookups struct{}

func (lookups) StatusName(_ context.Context, _, id string) (string, error) {
	return map[string]string{"todo": "To Do", "done": "Done"}[id], nil
}
func (lookups) SprintName(_ context.Context, id string) (string, string, error) {
	return "KYB", "Sprint " + id, nil
}

func groupSetup(adm admins) (*application.Service, *memory.GroupRepository, *memory.DeliveryRepository, *replies) {
	f := &fakes{
		issues:  map[string]application.IssueInfo{"KYB-1": {Project: "KYB", Title: "Login page"}, "KYB-9": {Project: "KYB", Title: "Salaries", Confidential: true}},
		members: map[string]bool{},
		names:   map[string]string{"alice": "Alice"},
	}
	groups, deliveries, bot := memory.NewGroupRepository(), memory.NewDeliveryRepository(), &replies{}
	n := 0
	return application.NewService(application.Deps{
		Repo: memory.NewRepository(), Issues: f, Members: f, Workspaces: f, Users: f, Audience: f,
		Deliveries: deliveries, Prefs: memory.NewPrefsRepository(), Contacts: contacts{}, Channels: []domain.Channel{domain.ChannelTelegram},
		PublicURL: "https://kyber.example", Groups: groups, Admins: adm, Bot: bot, BotName: "KyberBot", Lookups: lookups{},
		NewID: func() string { n++; return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) },
		Now:   func() time.Time { return clock },
	}), groups, deliveries, bot
}

func startPayload(t *testing.T, link string) string {
	t.Helper()
	u, _ := url.Parse(link)
	if u.Host != "t.me" || u.Path != "/KyberBot" {
		t.Fatalf("link = %q", link)
	}
	return u.Query().Get("startgroup")
}

func TestLinkGroupToProject(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	s, groups, _, bot := groupSetup(admins{"dev": application.ErrForbidden})
	if _, _, err := s.StartGroupLink(ctx, "dev", "KYB"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin err = %v", err)
	}
	link, expires, err := s.StartGroupLink(ctx, "alice", "KYB")
	if err != nil || !expires.Equal(clock.Add(domain.GroupTokenTTL)) {
		t.Fatalf("start = %q, %v", link, err)
	}
	p := startPayload(t, link)
	// The bot runs outside any request: the token carries the workspace.
	if err := s.OnGroupStart(context.Background(), p, "-100500", "Kyber devs"); err != nil {
		t.Fatal(err)
	}
	chats, _ := groups.ChatsOf(ctx, "KYB")
	if len(chats) != 1 || chats[0].ChatID != "-100500" || chats[0].Title != "Kyber devs" || chats[0].LinkedBy != "alice" {
		t.Fatalf("chats = %+v", chats)
	}
	if len(bot.sent) != 1 || !strings.Contains(bot.sent[0], "-100500: ✅ This group now gets KYB updates") {
		t.Fatalf("reply = %v", bot.sent)
	}
	if err := s.OnGroupStart(context.Background(), p, "-100500", "Kyber devs"); !errors.Is(err, domain.ErrGroupTokenInvalid) {
		t.Fatalf("reuse err = %v", err)
	}
	if !strings.Contains(bot.sent[1], "not valid any more") {
		t.Fatalf("reuse reply = %q", bot.sent[1])
	}
	list, err := s.GroupChats(ctx, "alice", "KYB")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if err := s.UnlinkGroup(ctx, "dev", "KYB", "-100500"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin unlink err = %v", err)
	}
	if err := s.UnlinkGroup(ctx, "alice", "KYB", "-100500"); err != nil {
		t.Fatal(err)
	}
}

// An admin who loses the role before the group is added cannot link it any more.
func TestGroupLinkRechecksAdmin(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	adm := admins{}
	s, groups, _, _ := groupSetup(adm)
	link, _, _ := s.StartGroupLink(ctx, "alice", "KYB")
	adm["alice"] = application.ErrForbidden
	if err := s.OnGroupStart(context.Background(), startPayload(t, link), "-1", "G"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	if chats, _ := groups.ChatsOf(ctx, "KYB"); len(chats) != 0 {
		t.Fatalf("linked anyway: %+v", chats)
	}
}

func TestProjectEventsGoToGroups(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	s, groups, deliveries, _ := groupSetup(admins{})
	_ = groups.LinkChat(ctx, domain.ProjectChat{Workspace: tenant.Default, Project: "KYB", ChatID: "-1", Title: "G", LinkedAt: clock})
	_ = groups.LinkChat(ctx, domain.ProjectChat{Workspace: tenant.Default, Project: "KYB", ChatID: "-2", Title: "H", LinkedAt: clock})
	events := []struct {
		name, payload string
	}{
		{"issue.created", `{"id":"i","key":"KYB-1","type":"task","title":"Login page","reporter":"alice"}`},
		{"issue.changed", `{"id":"i","key":"KYB-1","by":"alice","items":[{"field":"title","from":"a","to":"b"},{"field":"status","from":"todo","to":"done"}]}`},
		{"issue.changed", `{"id":"i","key":"KYB-1","by":"alice","items":[{"field":"title","from":"b","to":"c"}]}`}, // not a status change
		{"sprint.started", `{"id":"12","project":"KYB"}`},
		{"sprint.completed", `{"id":"12","project":"KYB"}`},
		{"issue.changed", `{"id":"j","key":"KYB-9","by":"alice","items":[{"field":"status","from":"todo","to":"done"}]}`}, // confidential
		{"issue.created", `garbage`},
	}
	for i, e := range events {
		m := msg(int64(i+1), e.name, e.payload)
		if err := s.OnProjectEvent(ctx, m); err != nil {
			t.Fatalf("%s: %v", e.name, err)
		}
		_ = s.OnProjectEvent(ctx, m) // redelivery
	}
	var got []string
	for _, d := range deliveries.All() {
		if d.To == "-1" {
			got = append(got, strings.SplitN(d.Body, "\n", 2)[0])
		}
	}
	want := "🆕 Alice created KYB-1 «Login page»|🔄 Alice moved KYB-1 «Login page»: To Do → Done|🏁 Sprint 12 started|✅ Sprint 12 completed"
	if strings.Join(got, "|") != want {
		t.Fatalf("group -1 got:\n%s\nwant:\n%s", strings.Join(got, "|"), want)
	}
	if len(deliveries.All()) != 8 {
		t.Fatalf("both groups, once each: %d", len(deliveries.All()))
	}
}

// Regression (S46 matrix): without a bot, the group list still enforces Administer first, so
// outsiders get 404 and members 403 instead of an empty list.
func TestGroupChatsAuthorizesEvenWithoutABot(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	s := application.NewService(application.Deps{Repo: memory.NewRepository(), Admins: admins{"eve": application.ErrNotFound, "dev": application.ErrForbidden},
		NewID: func() string { return "x" }, Now: func() time.Time { return clock }})
	for who, want := range map[string]error{"eve": application.ErrNotFound, "dev": application.ErrForbidden} {
		if _, err := s.GroupChats(ctx, who, "KYB"); !errors.Is(err, want) {
			t.Errorf("%s err = %v, want %v", who, err, want)
		}
	}
	if list, err := s.GroupChats(ctx, "alice", "KYB"); err != nil || len(list) != 0 {
		t.Fatalf("admin without bot = %v, %v", list, err)
	}
}
