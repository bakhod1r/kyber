package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	app "github.com/bakhod1r/kyber/internal/chat/application"
	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// members is a fake workspace directory: user -> workspaces.
type members map[string][]string

func (m members) IsMember(_ context.Context, ws, user string) (bool, error) {
	for _, w := range m[user] {
		if w == ws {
			return true, nil
		}
	}
	return false, nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { c.t = c.t.Add(time.Second); return c.t }

// events is a fake Notifier recording who was told what.
type events []string

func (e *events) Notify(_ context.Context, to []domain.UserID, ev domain.Event) {
	*e = append(*e, fmt.Sprintf("%s→%v", ev.EventName(), to))
}

func setup(t *testing.T) (*app.Service, context.Context) {
	svc, ctx, _ := setupWithEvents(t)
	return svc, ctx
}

func setupWithEvents(t *testing.T) (*app.Service, context.Context, *events) {
	t.Helper()
	repo := memory.NewRepository()
	n := 0
	newID := func() string { n++; return fmt.Sprintf("id-%03d", n) }
	c := &clock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	dir := members{"lead": {"w1"}, "dev": {"w1"}, "qa": {"w1"}, "eve": {"w2"}}
	ev := &events{}
	return app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Members: dir, NewID: newID, Now: c.now, Notifier: ev}),
		tenant.With(context.Background(), "w1"), ev
}

func TestChannels(t *testing.T) {
	svc, ctx := setup(t)
	general, err := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateChannel(ctx, "dev", app.NewChannel{Name: "General"}); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("duplicate = %v", err)
	}
	secret, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "secret", Private: true})

	// dev sees general (not joined) but not secret; cannot post before joining.
	list, _ := svc.Conversations(ctx, "dev")
	if len(list) != 1 || list[0].ID != general.ID || list[0].IsMember("dev") {
		t.Fatalf("dev list = %+v", list)
	}
	if _, err := svc.Post(ctx, "dev", string(general.ID), "hi"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("post before join = %v", err)
	}
	if _, err := svc.Conversation(ctx, "dev", string(secret.ID)); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("private read by outsider = %v (must look missing)", err)
	}
	if err := svc.Join(ctx, "dev", string(secret.ID)); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("join private = %v", err)
	}
	if err := svc.Join(ctx, "dev", string(general.ID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Invite(ctx, "lead", string(secret.ID), "eve"); !errors.Is(err, app.ErrNotWorkspaceMember) {
		t.Fatalf("invite outsider = %v", err)
	}
	if err := svc.Invite(ctx, "lead", string(secret.ID), "dev"); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.Conversations(ctx, "dev"); len(list) != 2 {
		t.Fatalf("after invite = %+v", list)
	}
	if err := svc.Leave(ctx, "dev", string(secret.ID)); err != nil {
		t.Fatal(err)
	}
	// Another workspace sees nothing of w1.
	other := tenant.With(context.Background(), "w2")
	if list, _ := svc.Conversations(other, "eve"); len(list) != 0 {
		t.Fatalf("w2 list = %+v", list)
	}
	if _, err := svc.Conversation(other, "eve", string(general.ID)); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("cross-workspace = %v", err)
	}
}

func TestDirectMessages(t *testing.T) {
	svc, ctx := setup(t)
	dm, err := svc.OpenDM(ctx, "dev", []string{"lead"})
	if err != nil {
		t.Fatal(err)
	}
	again, _ := svc.OpenDM(ctx, "lead", []string{"dev", "lead"})
	if again.ID != dm.ID {
		t.Fatalf("same people → same DM: %v vs %v", again.ID, dm.ID)
	}
	if _, err := svc.OpenDM(ctx, "dev", []string{"eve"}); !errors.Is(err, app.ErrNotWorkspaceMember) {
		t.Fatalf("DM outsider = %v", err)
	}
	if _, err := svc.Conversation(ctx, "qa", string(dm.ID)); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("third party = %v", err)
	}
}

func TestMessagesAndUnread(t *testing.T) {
	svc, ctx := setup(t)
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev"})
	_ = svc.Join(ctx, "dev", string(ch.ID))
	var ids []string
	for i := range 3 {
		m, err := svc.Post(ctx, "lead", string(ch.ID), fmt.Sprintf("m%d", i))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, string(m.ID))
	}
	if list, _ := svc.Conversations(ctx, "dev"); list[0].Unread != 3 {
		t.Fatalf("unread = %+v", list[0])
	}
	page, err := svc.History(ctx, "dev", string(ch.ID), "", 2)
	if err != nil || len(page) != 2 || page[0].Body != "m2" {
		t.Fatalf("history = %v %v", page, err)
	}
	if page, _ := svc.History(ctx, "dev", string(ch.ID), ids[1], 50); len(page) != 1 || page[0].Body != "m0" {
		t.Fatalf("before = %v", page)
	}
	if _, err := svc.History(ctx, "dev", string(ch.ID), "unknown", 50); !errors.Is(err, domain.ErrMessageNotFound) {
		t.Fatalf("unknown cursor = %v", err)
	}
	// Readers of a public channel may read history without joining.
	if page, err := svc.History(ctx, "qa", string(ch.ID), "", 0); err != nil || len(page) != 3 {
		t.Fatalf("qa history = %v %v (limit 0 = default)", page, err)
	}
	if err := svc.MarkRead(ctx, "dev", string(ch.ID)); err != nil {
		t.Fatal(err)
	}
	if list, _ := svc.Conversations(ctx, "dev"); list[0].Unread != 0 {
		t.Fatalf("after read = %+v", list[0])
	}
	if _, err := svc.Edit(ctx, "dev", ids[0], "x"); !errors.Is(err, domain.ErrNotAuthor) {
		t.Fatalf("edit other's = %v", err)
	}
	if _, err := svc.Edit(tenant.With(context.Background(), "w2"), "eve", ids[0], "x"); !errors.Is(err, domain.ErrMessageNotFound) {
		t.Fatalf("outsider edit = %v", err)
	}
	m, err := svc.Edit(ctx, "lead", ids[0], "m0 fixed")
	if err != nil || m.Body != "m0 fixed" || m.EditedAt.IsZero() {
		t.Fatalf("edit = %+v %v", m, err)
	}
	if err := svc.Delete(ctx, "lead", ids[0]); err != nil {
		t.Fatal(err)
	}
	if page, _ := svc.History(ctx, "dev", string(ch.ID), "", 50); !page[2].Deleted {
		t.Fatalf("deleted kept in history = %+v", page[2])
	}
	if _, err := svc.Post(ctx, "lead", string(ch.ID), " "); !errors.Is(err, domain.ErrInvalidBody) {
		t.Fatalf("blank = %v", err)
	}
}

func TestRealtimeEvents(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev"})
	_ = svc.Join(ctx, "dev", string(ch.ID))
	m, _ := svc.Post(ctx, "lead", string(ch.ID), "hi")
	_, _ = svc.Edit(ctx, "lead", string(m.ID), "hello")
	_ = svc.Delete(ctx, "lead", string(m.ID))
	if err := svc.Typing(ctx, "dev", string(ch.ID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Typing(ctx, "qa", string(ch.ID)); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("typing outside = %v", err)
	}
	want := []string{"message.posted→[dev lead]", "message.edited→[dev lead]", "message.deleted→[dev lead]", "typing→[lead]"}
	if fmt.Sprint(*ev) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", *ev, want)
	}
}

func TestThreadsAndReactions(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev"})
	_ = svc.Join(ctx, "dev", string(ch.ID))
	parent, _ := svc.Post(ctx, "lead", string(ch.ID), "release today?")
	other, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "other"})
	foreign, _ := svc.Post(ctx, "lead", string(other.ID), "elsewhere")

	r, err := svc.Reply(ctx, "dev", string(parent.ID), "yes")
	if err != nil || r.Thread != parent.ID {
		t.Fatalf("reply = %+v %v", r, err)
	}
	if _, err := svc.Reply(ctx, "dev", string(r.ID), "nested"); !errors.Is(err, domain.ErrNestedThread) {
		t.Fatalf("nested = %v", err)
	}
	if _, err := svc.Reply(ctx, "dev", string(foreign.ID), "not a member"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("reply without membership = %v", err)
	}
	thread, err := svc.Replies(ctx, "qa", string(parent.ID))
	if err != nil || len(thread) != 1 {
		t.Fatalf("replies = %v %v", thread, err)
	}
	if top, _ := svc.History(ctx, "dev", string(ch.ID), "", 0); len(top) != 1 || top[0].ReplyCount != 1 {
		t.Fatalf("history = %+v", top)
	}

	m, err := svc.React(ctx, "dev", string(parent.ID), "🎉")
	if err != nil || len(m.Reactions) != 1 {
		t.Fatalf("react = %+v %v", m, err)
	}
	if _, err := svc.React(ctx, "qa", string(parent.ID), "🎉"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("react without joining = %v", err)
	}
	if m, _ := svc.React(ctx, "dev", string(parent.ID), "🎉"); len(m.Reactions) != 0 {
		t.Fatalf("toggle off = %+v", m.Reactions)
	}
	if last := (*ev)[len(*ev)-1]; last != "message.reacted→[dev lead]" {
		t.Fatalf("last event = %s", last)
	}
	if !slices.Contains(*ev, "message.posted→[dev lead]") {
		t.Fatalf("events = %v", *ev)
	}
}

// issues is a fake Issue Tracking port: key → preview and who may see it.
type issues map[string]struct {
	app.IssuePreview
	viewers []string
}

func (f issues) Preview(_ context.Context, user, key string) (app.IssuePreview, bool, error) {
	i, ok := f[key]
	if !ok || !slices.Contains(i.viewers, user) {
		return app.IssuePreview{}, false, nil
	}
	return i.IssuePreview, true, nil
}

func TestIssuePreviews(t *testing.T) {
	svc, ctx := setup(t)
	svc.WithIssues(issues{
		"KYB-1": {app.IssuePreview{Key: "KYB-1", Title: "Login", Status: "In Progress"}, []string{"lead", "dev"}},
		"HR-2":  {app.IssuePreview{Key: "HR-2", Title: "Salaries", Status: "To Do"}, []string{"lead"}},
	})
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev"})
	m1, _ := svc.Post(ctx, "lead", string(ch.ID), "KYB-1 and HR-2 are blocked")
	m2, _ := svc.Post(ctx, "lead", string(ch.ID), "no issues here, and NOPE-9")
	msgs := []*domain.Message{m1, m2}

	dev, err := svc.Previews(ctx, "dev", msgs)
	if err != nil || len(dev[m1.ID]) != 1 || dev[m1.ID][0].Title != "Login" || len(dev[m2.ID]) != 0 {
		t.Fatalf("dev previews = %v %v (HR-2 hidden)", dev, err)
	}
	if lead, _ := svc.Previews(ctx, "lead", msgs); len(lead[m1.ID]) != 2 || lead[m1.ID][1].Status != "To Do" {
		t.Fatalf("lead previews = %v", lead)
	}
}

func TestHuddleSystemMessages(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev"})
	started := outbox.Message{ID: 1, Name: "call.started", Workspace: "w1",
		Payload: []byte(`{"call_id":"k-1","conversation_id":"` + string(ch.ID) + `","by":"lead"}`)}
	ended := outbox.Message{ID: 2, Name: "call.ended", Workspace: "w1",
		Payload: []byte(`{"call_id":"k-1","conversation_id":"` + string(ch.ID) + `","by":"lead","seconds":725}`)}
	for _, m := range []outbox.Message{started, started, ended} { // redelivery is harmless
		if err := svc.OnCallEvent(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	page, _ := svc.History(ctx, "lead", string(ch.ID), "", 0)
	if len(page) != 2 || page[1].Kind != domain.KindHuddleStarted || page[1].Body != "started a huddle" ||
		page[0].Kind != domain.KindHuddleEnded || page[0].Body != "The huddle ended after 12 min" {
		t.Fatalf("history = %+v", page)
	}
	if !slices.Contains(*ev, "message.posted→[lead]") {
		t.Fatalf("members were not told: %v", *ev)
	}
	for _, bad := range []outbox.Message{{ID: 3, Name: "call.started", Payload: []byte("{")},
		{ID: 4, Name: "call.started", Payload: []byte(`{"call_id":"k-2","conversation_id":"gone","by":"lead"}`)}} {
		if err := svc.OnCallEvent(ctx, bad); err != nil {
			t.Fatalf("%s must be skipped: %v", bad.Payload, err)
		}
	}
}

func TestPinsAndSearch(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	pub, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	priv, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "secret", Private: true})
	_ = svc.Join(ctx, "dev", string(pub.ID))
	m, _ := svc.Post(ctx, "lead", string(pub.ID), "Release checklist v2")
	_, _ = svc.Post(ctx, "lead", string(priv.ID), "release salaries")

	if _, err := svc.Pin(ctx, "qa", string(m.ID)); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("pin without joining = %v", err)
	}
	pinned, err := svc.Pin(ctx, "dev", string(m.ID))
	if err != nil || pinned.PinnedBy != "dev" {
		t.Fatalf("pin = %+v %v", pinned, err)
	}
	if last := (*ev)[len(*ev)-1]; last != "message.pinned→[dev lead]" {
		t.Fatalf("event = %s", last)
	}
	if pins, err := svc.Pins(ctx, "qa", string(pub.ID)); err != nil || len(pins) != 1 {
		t.Fatalf("pins (public, read by a non-member) = %v %v", pins, err)
	}
	if _, err := svc.Unpin(ctx, "dev", string(m.ID)); err != nil {
		t.Fatal(err)
	}
	if pins, _ := svc.Pins(ctx, "dev", string(pub.ID)); len(pins) != 0 {
		t.Fatalf("after unpin = %v", pins)
	}

	// Search covers what the reader may read: dev finds the public message, not the private channel's.
	if found, err := svc.Search(ctx, "dev", "release"); err != nil || len(found) != 1 || found[0].ID != m.ID {
		t.Fatalf("dev search = %v %v", found, err)
	}
	if found, _ := svc.Search(ctx, "lead", "release"); len(found) != 2 {
		t.Fatalf("lead search = %v", found)
	}
	if _, err := svc.Search(ctx, "dev", " r "); !errors.Is(err, app.ErrQueryTooShort) {
		t.Fatalf("short query = %v", err)
	}
}
