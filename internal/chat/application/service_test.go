package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	app "github.com/bakhod1r/kyber/internal/chat/application"
	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/blob"
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

func TestFiles(t *testing.T) {
	repo := memory.NewRepository()
	blobs := blob.NewMemory()
	n := 0
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Members: members{"lead": {"w1"}, "dev": {"w1"}, "qa": {"w1"}},
		Blobs: blobs, NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "dev", Private: true})

	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 100)...)
	m, err := svc.Upload(ctx, "lead", string(ch.ID), "shot.png", bytes.NewReader(png))
	if err != nil || m.File == nil || m.File.Type != "image/png" || m.File.Size != int64(len(png)) {
		t.Fatalf("upload = %+v %+v %v (type sniffed, size counted)", m, m.File, err)
	}
	f, r, err := svc.Download(ctx, "lead", string(m.ID))
	if err != nil || f.Name != "shot.png" {
		t.Fatalf("download = %+v %v", f, err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, png) {
		t.Fatal("bytes differ")
	}
	if _, _, err := svc.Download(ctx, "qa", string(m.ID)); !errors.Is(err, domain.ErrMessageNotFound) {
		t.Fatalf("outsider download = %v", err)
	}
	if _, err := svc.Upload(ctx, "qa", string(ch.ID), "x.txt", strings.NewReader("x")); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("outsider upload = %v", err)
	}
	big := io.LimitReader(zeros{}, domain.MaxFile+1)
	if _, err := svc.Upload(ctx, "lead", string(ch.ID), "big.bin", big); !errors.Is(err, domain.ErrInvalidFile) {
		t.Fatalf("too large = %v", err)
	}
	if _, err := blobs.Open(ctx, "id-004"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("a refused upload leaves no blob: %v", err)
	}
	if err := svc.Delete(ctx, "lead", string(m.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := blobs.Open(ctx, string(m.ID)); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("deleting the message deletes the file: %v", err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// S65: save for later, with a reminder sent once by the background job.
func TestSavedForLater(t *testing.T) {
	repo := memory.NewRepository()
	saved := memory.NewSaved(repo)
	n := 0
	c := &clock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Saved: saved, Members: members{"lead": {"w1"}, "dev": {"w1"}, "eve": {"w1"}},
		NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: c.now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	secret, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "secret", Private: true})
	m, _ := svc.Post(ctx, "lead", string(ch.ID), "check the deploy")
	hidden, _ := svc.Post(ctx, "lead", string(secret.ID), "psst")

	if _, err := svc.SaveMessage(ctx, "dev", string(hidden.ID), time.Time{}); !errors.Is(err, domain.ErrMessageNotFound) {
		t.Fatalf("saving an unreadable message = %v", err)
	}
	if _, err := svc.SaveMessage(ctx, "dev", string(m.ID), c.t.Add(-time.Minute)); !errors.Is(err, domain.ErrInvalidReminder) {
		t.Fatalf("reminder in the past = %v", err)
	}
	if _, err := svc.SaveMessage(ctx, "dev", string(m.ID), c.t.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	list, err := svc.SavedFor(ctx, "dev")
	if err != nil || len(list) != 1 || list[0].Message.Body != "check the deploy" || list[0].Saved.RemindAt.IsZero() {
		t.Fatalf("saved = %+v %v", list, err)
	}
	if l, _ := svc.SavedFor(ctx, "eve"); len(l) != 0 {
		t.Fatalf("eve's saves = %+v", l)
	}

	if sent, _ := svc.RemindDue(context.Background()); sent != 0 {
		t.Fatalf("reminded early: %d", sent)
	}
	c.t = c.t.Add(time.Hour)
	if sent, err := svc.RemindDue(context.Background()); sent != 1 || err != nil {
		t.Fatalf("due reminders = %d %v", sent, err)
	}
	if sent, _ := svc.RemindDue(context.Background()); sent != 0 {
		t.Fatalf("reminded twice: %d", sent)
	}
	out, meta := saved.Outbox(), saved.OutboxMeta()
	if len(out) != 1 || out[0].User != "dev" || out[0].Message != m.ID || meta[0].Workspace != "w1" {
		t.Fatalf("outbox = %+v %+v", out, meta)
	}

	if err := svc.Unsave(ctx, "dev", string(m.ID)); err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.SavedFor(ctx, "dev"); len(l) != 0 {
		t.Fatalf("after unsave = %+v", l)
	}
}

// S66: statuses and Do Not Disturb, visible to the workspace.
func TestStatuses(t *testing.T) {
	c := &clock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Statuses: memory.NewStatuses(),
		Members: members{"lead": {"w1"}, "dev": {"w1"}, "eve": {"w2"}}, NewID: func() string { return "x" }, Now: c.now})
	ctx := tenant.With(context.Background(), "w1")
	if _, err := svc.SetStatus(ctx, "lead", "🏖", "On vacation", c.t.Add(2*time.Hour), c.t.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Statuses(ctx, "dev", []string{"lead", "dev"})
	if err != nil || len(got) != 1 || got["lead"].Text != "On vacation" || !got["lead"].DND {
		t.Fatalf("statuses = %+v %v", got, err)
	}
	if quiet, _ := svc.InDND(ctx, "lead"); !quiet {
		t.Fatal("lead not in DND")
	}
	if _, err := svc.Statuses(tenant.With(context.Background(), "w1"), "eve", []string{"lead"}); !errors.Is(err, app.ErrNotWorkspaceMember) {
		t.Fatalf("outsider = %v", err)
	}
	c.t = c.t.Add(90 * time.Minute)
	if quiet, _ := svc.InDND(ctx, "lead"); quiet {
		t.Fatal("DND did not end")
	}
	c.t = c.t.Add(time.Hour)
	if got, _ := svc.Statuses(ctx, "dev", []string{"lead"}); len(got) != 0 {
		t.Fatalf("expired status = %+v", got)
	}
}

// S67: scheduled messages are posted by the background job at their time; only the author sees them before.
func TestScheduledMessages(t *testing.T) {
	repo := memory.NewRepository()
	sched, rem := memory.NewScheduled(), memory.NewReminders()
	n := 0
	c := &clock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	ev := &events{}
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Scheduled: sched, Reminders: rem, Notifier: ev,
		Members: members{"lead": {"w1"}, "dev": {"w1"}}, NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: c.now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	if _, err := svc.Schedule(ctx, "dev", string(ch.ID), "hi", c.t.Add(time.Hour)); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("not a member = %v", err)
	}
	s, err := svc.Schedule(ctx, "lead", string(ch.ID), "standup in 5", c.t.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reschedule(ctx, "dev", string(s.ID), "x", c.t.Add(time.Hour)); !errors.Is(err, domain.ErrScheduledNotFound) {
		t.Fatalf("someone else's = %v", err)
	}
	if _, err := svc.Reschedule(ctx, "lead", string(s.ID), "standup in 5 min", c.t.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if mine, _ := svc.ScheduledFor(ctx, "lead"); len(mine) != 1 || mine[0].Body != "standup in 5 min" {
		t.Fatalf("mine = %+v", mine)
	}
	if h, _ := svc.History(ctx, "lead", string(ch.ID), "", 50); len(h) != 0 {
		t.Fatalf("posted early: %+v", h)
	}
	c.t = c.t.Add(2 * time.Hour)
	if sent, err := svc.SendDue(context.Background()); sent != 1 || err != nil {
		t.Fatalf("send due = %d %v", sent, err)
	}
	h, _ := svc.History(ctx, "lead", string(ch.ID), "", 50)
	if len(h) != 1 || h[0].Body != "standup in 5 min" || h[0].Author != "lead" {
		t.Fatalf("history = %+v", h)
	}
	if mine, _ := svc.ScheduledFor(ctx, "lead"); len(mine) != 0 {
		t.Fatalf("still scheduled: %+v", mine)
	}
	if sent, _ := svc.SendDue(context.Background()); sent != 0 {
		t.Fatalf("sent twice: %d", sent)
	}
	if len(repo.Outbox()) != 1 {
		t.Fatalf("mentions event = %+v", repo.Outbox())
	}

	// Cancel before it is sent.
	x, _ := svc.Schedule(ctx, "lead", string(ch.ID), "never", c.t.Add(time.Hour))
	if err := svc.CancelScheduled(ctx, "lead", string(x.ID)); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(2 * time.Hour)
	if sent, _ := svc.SendDue(context.Background()); sent != 0 {
		t.Fatalf("cancelled one sent: %d", sent)
	}

	// /remind me: a personal reminder, told as a notification.
	if _, err := svc.Remind(ctx, "dev", string(ch.ID), "call the bank", c.t.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(2 * time.Minute)
	if sent, err := svc.RemindDue(context.Background()); sent != 1 || err != nil {
		t.Fatalf("reminders = %d %v", sent, err)
	}
	if out := rem.Outbox(); len(out) != 1 || out[0].User != "dev" || out[0].Excerpt != "call the bank" {
		t.Fatalf("reminder outbox = %+v", out)
	}
}

// S68: in a DM the others learn how far I have read ("Seen"); channels keep it private.
func TestReadReceipts(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	dm, _ := svc.OpenDM(ctx, "lead", []string{"dev"})
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	_, _ = svc.Post(ctx, "lead", string(dm.ID), "hi")
	*ev = nil
	if err := svc.MarkRead(ctx, "dev", string(dm.ID)); err != nil {
		t.Fatal(err)
	}
	if len(*ev) != 1 || (*ev)[0] != "conversation.read→[lead]" {
		t.Fatalf("events = %v", *ev)
	}
	got, err := svc.Receipts(ctx, "lead", string(dm.ID))
	if err != nil || len(got) != 2 || got["dev"].IsZero() {
		t.Fatalf("receipts = %v %v (lead read it by posting)", got, err)
	}
	*ev = nil
	_ = svc.MarkRead(ctx, "lead", string(ch.ID))
	if len(*ev) != 0 {
		t.Fatalf("channel read announced: %v", *ev)
	}
	if got, _ := svc.Receipts(ctx, "lead", string(ch.ID)); len(got) != 0 {
		t.Fatalf("channel receipts = %v", got)
	}
	if _, err := svc.Receipts(ctx, "qa", string(dm.ID)); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("outsider = %v", err)
	}
}

// S69: owners and admins rename, archive and remove; an archived channel is read-only.
func TestChannelManagement(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	id := string(ch.ID)
	_ = svc.Join(ctx, "dev", id)
	_ = svc.Join(ctx, "qa", id)
	if _, err := svc.Rename(ctx, "dev", id, "team", ""); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("member renames = %v", err)
	}
	if _, err := svc.MakeAdmin(ctx, "lead", id, "dev"); err != nil {
		t.Fatal(err)
	}
	*ev = nil
	c, err := svc.Rename(ctx, "dev", id, "team", "The team")
	if err != nil || c.Name != "team" || len(*ev) != 1 || !strings.HasPrefix((*ev)[0], "conversation.updated→") {
		t.Fatalf("rename = %+v %v %v", c, err, *ev)
	}
	if _, err := svc.RemoveMember(ctx, "dev", id, "qa"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Post(ctx, "qa", id, "still here?"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("removed member posts = %v", err)
	}
	if _, err := svc.Archive(ctx, "dev", id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Post(ctx, "lead", id, "hello?"); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("post to archived = %v", err)
	}
	if h, err := svc.History(ctx, "lead", id, "", 10); err != nil || h == nil && err != nil {
		t.Fatalf("reading an archived channel = %v", err)
	}
	if _, err := svc.Unarchive(ctx, "lead", id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Post(ctx, "lead", id, "back"); err != nil {
		t.Fatalf("post after unarchive = %v", err)
	}
	if _, err := svc.RemoveAdmin(ctx, "dev", id, "dev"); !errors.Is(err, domain.ErrNotOwner) {
		t.Fatalf("admin removes admin = %v", err)
	}
}

// S70: user groups of a workspace.
func TestUserGroups(t *testing.T) {
	repo := memory.NewRepository()
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Groups: memory.NewGroups(),
		Members: members{"lead": {"w1"}, "dev": {"w1"}, "qa": {"w1"}, "eve": {"w2"}}, NewID: func() string { return "g-1" }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	if _, err := svc.CreateGroup(ctx, "lead", "backend", "Backend", []string{"dev", "eve"}); !errors.Is(err, app.ErrNotWorkspaceMember) {
		t.Fatalf("outsider as member = %v", err)
	}
	g, err := svc.CreateGroup(ctx, "lead", "backend", "Backend", []string{"dev", "lead"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateGroup(ctx, "qa", string(g.ID), "backend", "Backend", []string{"dev", "qa"}); err != nil {
		t.Fatalf("a workspace member edits = %v", err)
	}
	if ids, _ := svc.GroupMembers(ctx, "backend"); len(ids) != 2 || ids[1] != "qa" {
		t.Fatalf("members = %v", ids)
	}
	if list, _ := svc.Groups(ctx, "dev"); len(list) != 1 {
		t.Fatalf("groups = %+v", list)
	}
	if _, err := svc.Groups(ctx, "eve"); !errors.Is(err, app.ErrNotWorkspaceMember) {
		t.Fatalf("outsider lists = %v", err)
	}
	if err := svc.DeleteGroup(ctx, "dev", string(g.ID)); !errors.Is(err, domain.ErrNotOwner) {
		t.Fatalf("non-creator deletes = %v", err)
	}
	if err := svc.DeleteGroup(ctx, "lead", string(g.ID)); err != nil {
		t.Fatal(err)
	}
	if ids, _ := svc.GroupMembers(ctx, "backend"); len(ids) != 0 {
		t.Fatalf("after delete = %v", ids)
	}
}

// S71: per-person level and section; Notify asks who follows everything and who muted.
func TestConversationPrefs(t *testing.T) {
	repo := memory.NewRepository()
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Prefs: memory.NewPrefs(),
		Members: members{"lead": {"w1"}, "dev": {"w1"}, "qa": {"w1"}}, NewID: func() string { return "c-1" }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	id := string(ch.ID)
	_ = svc.Join(ctx, "dev", id)
	if _, err := svc.SetPref(ctx, "dev", id, domain.LevelAll, "Work"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPref(ctx, "qa", id, domain.LevelAll, ""); err != nil {
		t.Fatal(err) // qa reads the public channel without joining; following needs membership (checked below)
	}
	if _, err := svc.SetPref(ctx, "lead", id, domain.LevelMuted, ""); err != nil {
		t.Fatal(err)
	}
	if p, _ := svc.PrefsFor(ctx, "dev"); len(p) != 1 || p[0].Section != "Work" {
		t.Fatalf("dev's prefs = %+v", p)
	}
	if f, _ := svc.Followers(ctx, id); len(f) != 1 || f[0] != "dev" {
		t.Fatalf("followers = %v (members with level all)", f)
	}
	if m, _ := svc.Muted(ctx, "lead", id); !m {
		t.Fatal("lead muted it")
	}
	if m, _ := svc.Muted(ctx, "dev", id); m {
		t.Fatal("dev did not mute")
	}
}

// projects is a fake Projects port: who may browse which project.
type projects map[string][]string

func (p projects) CanBrowse(_ context.Context, user, key string) (bool, error) {
	return slices.Contains(p[key], user), nil
}

// S73: a project's news land in the channels linked to it, once each.
func TestProjectChannels(t *testing.T) {
	repo := memory.NewRepository()
	n := 0
	ev := &events{}
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Notifier: ev, Projects: projects{"KYB": {"lead"}},
		Members: members{"lead": {"w1"}, "dev": {"w1"}}, NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "kyb-news"})
	other, _ := svc.CreateChannel(ctx, "dev", app.NewChannel{Name: "dev-stuff"})
	if _, err := svc.LinkProject(ctx, "dev", string(other.ID), "KYB"); !errors.Is(err, app.ErrCannotBrowse) {
		t.Fatalf("linking a project dev cannot browse = %v", err)
	}
	if _, err := svc.LinkProject(ctx, "lead", string(ch.ID), "KYB"); err != nil {
		t.Fatal(err)
	}
	created := outbox.Message{ID: 7, Name: "issue.created", Workspace: "w1", Actor: "lead",
		Payload: []byte(`{"key":"KYB-12","title":"secret title","reporter":"lead"}`)}
	for range 2 { // redelivery posts once
		if err := svc.OnProjectEvent(ctx, created); err != nil {
			t.Fatal(err)
		}
	}
	moved := outbox.Message{ID: 8, Name: "issue.transitioned", Workspace: "w1", Actor: "dev", Payload: []byte(`{"key":"KYB-12","to":"done"}`)}
	sprint := outbox.Message{ID: 9, Name: "sprint.started", Workspace: "w1", Actor: "lead", Payload: []byte(`{"project":"KYB"}`)}
	elsewhere := outbox.Message{ID: 10, Name: "issue.created", Workspace: "w1", Payload: []byte(`{"key":"ABC-1","reporter":"lead"}`)}
	for _, m := range []outbox.Message{moved, sprint, elsewhere} {
		if err := svc.OnProjectEvent(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := svc.History(ctx, "lead", string(ch.ID), "", 50)
	if len(h) != 3 {
		t.Fatalf("history = %+v", h)
	}
	bodies := []string{h[2].Body, h[1].Body, h[0].Body}
	if bodies[0] != "created KYB-12" || bodies[1] != "moved KYB-12 to done" || bodies[2] != "started a sprint in KYB" || h[2].Kind != domain.KindProjectEvent {
		t.Fatalf("bodies = %q", bodies)
	}
	for _, b := range bodies {
		if strings.Contains(b, "secret") {
			t.Fatalf("issue details leaked into the channel: %q", b)
		}
	}
}

// transcriber is a fake Transcriber.
type transcriber struct{ calls int }

func (t *transcriber) Transcribe(_ context.Context, name string, audio io.Reader) (string, error) {
	t.calls++
	b, _ := io.ReadAll(audio)
	return fmt.Sprintf("transcript of %s (%d bytes)", name, len(b)), nil
}

// S77: with a speech-to-text service configured, a recording gets its transcript in its thread, once.
func TestRecordingTranscript(t *testing.T) {
	repo := memory.NewRepository()
	n := 0
	tr := &transcriber{}
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Blobs: blob.NewMemory(), Transcriber: tr,
		Members: members{"lead": {"w1"}}, NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	rec, err := svc.UploadRecording(ctx, "lead", string(ch.ID), "huddle.webm", bytes.NewReader(append([]byte("\x1aE\xdf\xa3"), make([]byte, 100)...)))
	if err != nil {
		t.Fatal(err)
	}
	out := repo.Outbox()
	if len(out) != 1 || out[0].Kind != domain.KindHuddleRecording {
		t.Fatalf("outbox = %+v", out)
	}
	payload, _ := json.Marshal(out[0])
	m := outbox.Message{ID: 1, Name: out[0].EventName(), Payload: payload, Workspace: "w1"}
	for range 2 {
		if err := svc.OnRecordingPosted(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	replies, _ := svc.Replies(ctx, "lead", string(rec.ID))
	if tr.calls != 1 || len(replies) != 1 || !strings.HasPrefix(replies[0].Body, "📝 Transcript:\ntranscript of huddle.webm") || replies[0].Kind != domain.KindTranscript {
		t.Fatalf("calls %d, replies = %+v", tr.calls, replies)
	}
	// Without a service nothing leaves the server.
	plain := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Blobs: blob.NewMemory(), Members: members{}, NewID: func() string { return "x" }, Now: time.Now})
	if err := plain.OnRecordingPosted(ctx, m); err != nil {
		t.Fatal(err)
	}
}

// S79: members keep docs and links in the channel's header; the one who added it or a manager removes it.
func TestBookmarks(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	svc = svc.WithBookmarks(memory.NewBookmarks())
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	id := string(ch.ID)
	if _, err := svc.AddBookmark(ctx, "dev", id, domain.BookmarkLink, "https://ci.example", "CI"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("non-member adds = %v", err)
	}
	_ = svc.Join(ctx, "dev", id)
	_ = svc.Join(ctx, "qa", id)
	*ev = nil
	b, err := svc.AddBookmark(ctx, "dev", id, domain.BookmarkLink, "https://ci.example", "CI")
	if err != nil || len(*ev) != 1 {
		t.Fatalf("add = %+v %v %v", b, err, *ev)
	}
	if list, _ := svc.Bookmarks(ctx, "qa", id); len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if err := svc.RemoveBookmark(ctx, "qa", string(b.ID)); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("someone else removes = %v", err)
	}
	if err := svc.RemoveBookmark(ctx, "lead", string(b.ID)); err != nil {
		t.Fatalf("owner removes = %v", err)
	}
	for i := 0; i < domain.MaxBookmarks; i++ {
		if _, err := svc.AddBookmark(ctx, "dev", id, domain.BookmarkLink, "https://x.example", "x"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.AddBookmark(ctx, "dev", id, domain.BookmarkLink, "https://x.example", "x"); !errors.Is(err, domain.ErrTooManyBookmarks) {
		t.Fatalf("21st = %v", err)
	}
}

// names is a fake Directory.
type names map[string]string

func (n names) DisplayName(_ context.Context, u string) (string, error) { return n[u], nil }

// S80: workflows — a welcome on join, a message on a schedule (once per occurrence).
func TestWorkflows(t *testing.T) {
	repo := memory.NewRepository()
	n := 0
	c := &clock{t: time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)} // Tuesday
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Workflows: memory.NewWorkflows(), Directory: names{"dev": "Dev"},
		Members: members{"lead": {"w1"}, "dev": {"w1"}}, NewID: func() string { n++; return fmt.Sprintf("id-%03d", n) }, Now: c.now})
	ctx := tenant.With(context.Background(), "w1")
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "general"})
	id := string(ch.ID)
	if _, err := svc.AddWorkflow(ctx, "dev", id, domain.OnJoin, 0, 0, "hi"); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("non-manager adds = %v", err)
	}
	if _, err := svc.AddWorkflow(ctx, "lead", id, domain.OnJoin, 0, 0, "Welcome {name}! Read the pinned notes."); err != nil {
		t.Fatal(err)
	}
	daily, err := svc.AddWorkflow(ctx, "lead", id, domain.OnSchedule, 9*60+30, domain.Weekdays, "Standup in the huddle")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Join(ctx, "dev", id); err != nil {
		t.Fatal(err)
	}
	h, _ := svc.History(ctx, "lead", id, "", 50)
	if len(h) != 1 || h[0].Body != "Welcome Dev! Read the pinned notes." || h[0].Kind != domain.KindWorkflow {
		t.Fatalf("after join = %+v", h)
	}
	c.t = time.Date(2026, 10, 13, 9, 31, 0, 0, time.UTC)
	for range 2 {
		if _, err := svc.RunWorkflows(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	h, _ = svc.History(ctx, "lead", id, "", 50)
	if len(h) != 2 || h[0].Body != "Standup in the huddle" {
		t.Fatalf("after 09:30 = %+v", h)
	}
	if list, _ := svc.Workflows(ctx, "dev", id); len(list) != 2 {
		t.Fatalf("workflows = %+v", list)
	}
	if err := svc.RemoveWorkflow(ctx, "dev", string(daily.ID)); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("non-manager removes = %v", err)
	}
	if err := svc.RemoveWorkflow(ctx, "lead", string(daily.ID)); err != nil {
		t.Fatal(err)
	}
}

// S81: incoming webhooks post into a channel with a secret URL; managers create and rotate them.
func TestWebhooks(t *testing.T) {
	svc, ctx, ev := setupWithEvents(t)
	svc = svc.WithWebhooks(memory.NewWebhooks())
	ch, _ := svc.CreateChannel(ctx, "lead", app.NewChannel{Name: "builds"})
	id := string(ch.ID)
	_ = svc.Join(ctx, "dev", id)
	if _, _, err := svc.CreateWebhook(ctx, "dev", id, "CI"); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("member creates = %v", err)
	}
	h, secret, err := svc.CreateWebhook(ctx, "lead", id, "CI")
	if err != nil || len(secret) < 32 {
		t.Fatalf("create = %+v %q %v", h, secret, err)
	}
	*ev = nil
	m, err := svc.PostViaWebhook(context.Background(), secret, "build #42 passed")
	if err != nil || m.Body != "CI: build #42 passed" || m.Kind != domain.KindWebhook || len(*ev) != 1 {
		t.Fatalf("post = %+v %v %v", m, err, *ev)
	}
	if _, err := svc.PostViaWebhook(context.Background(), "wrong", "x"); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("wrong secret = %v", err)
	}
	fresh, err := svc.RotateWebhook(ctx, "lead", string(h.ID))
	if err != nil || fresh == secret {
		t.Fatalf("rotate = %q %v", fresh, err)
	}
	if _, err := svc.PostViaWebhook(context.Background(), secret, "x"); !errors.Is(err, domain.ErrWebhookNotFound) {
		t.Fatalf("old secret after rotate = %v", err)
	}
	for i := 0; i < app.WebhookRate-1; i++ {
		if _, err := svc.PostViaWebhook(context.Background(), fresh, "spam"); err != nil {
			t.Fatalf("message %d = %v", i, err)
		}
	}
	if _, err := svc.PostViaWebhook(context.Background(), fresh, "spam"); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("over the rate = %v", err)
	}
	if list, _ := svc.Webhooks(ctx, "lead", id); len(list) != 1 {
		t.Fatalf("hooks = %+v", list)
	}
	if err := svc.RemoveWebhook(ctx, "lead", string(h.ID)); err != nil {
		t.Fatal(err)
	}
}

// botWorld fakes Identity (making bots) and Workspace (admins, joining).
type botWorld struct {
	admins map[string]bool
	joined []string
	n      int
}

func (w *botWorld) MakeBot(_ context.Context, name string) (string, string, error) {
	w.n++
	return fmt.Sprintf("bot-%d", w.n), "kpat_secret", nil
}
func (w *botWorld) Administers(_ context.Context, ws, user string) (bool, error) { return w.admins[user], nil }
func (w *botWorld) Join(_ context.Context, ws, user string) error {
	w.joined = append(w.joined, ws+"/"+user)
	return nil
}

// S84: workspace admins make bots; a bot joins the workspace and shows 🤖.
func TestBots(t *testing.T) {
	repo := memory.NewRepository()
	w := &botWorld{admins: map[string]bool{"lead": true}}
	statuses := memory.NewStatuses()
	svc := app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Bots: memory.NewBots(), BotMaker: w, Workspace: w, Statuses: statuses,
		Members: members{"lead": {"w1"}, "dev": {"w1"}, "bot-1": {"w1"}}, NewID: func() string { return "x" }, Now: time.Now})
	ctx := tenant.With(context.Background(), "w1")
	if _, _, err := svc.CreateBot(ctx, "dev", "CI"); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("non-admin = %v", err)
	}
	b, token, err := svc.CreateBot(ctx, "lead", "CI")
	if err != nil || token != "kpat_secret" || b.User != "bot-1" || len(w.joined) != 1 || w.joined[0] != "w1/bot-1" {
		t.Fatalf("bot = %+v %q %v %v", b, token, err, w.joined)
	}
	if st, _ := svc.Statuses(ctx, "dev", []string{"bot-1"}); st["bot-1"].Emoji != "🤖" {
		t.Fatalf("status = %+v", st)
	}
	if list, _ := svc.BotsIn(ctx, "dev"); len(list) != 1 || list[0].Name != "CI" {
		t.Fatalf("bots = %+v", list)
	}
}
