// Package repotest is the contract every notification Repository adapter must satisfy.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// Users that adapters must accept (the Postgres factory inserts them).
const (
	Alice = "90000000-0000-4000-8000-00000000000a"
	Bob   = "90000000-0000-4000-8000-00000000000b"
)

// Workspaces used by the contract (plain ids; notifications do not reference the workspace table).
const (
	WS    = "00000000-0000-4000-8000-000000000001"
	Other = "92000000-0000-4000-8000-000000000001"
)

func inbox(u string) domain.Inbox { return domain.Inbox{Workspace: WS, User: domain.UserID(u)} }

func nid(n int) domain.NotificationID {
	return domain.NotificationID("91000000-0000-4000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

func Run(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	mk := func(n int, to string, event int64) domain.Notification {
		return domain.Notification{ID: nid(n), Recipient: domain.UserID(to), Workspace: WS, Kind: domain.KindAssigned, IssueKey: "KYB-1",
			IssueTitle: "Login", ActorName: "Carol", Excerpt: "x", SourceEvent: event, CreatedAt: t0.Add(time.Duration(n) * time.Minute)}
	}

	t.Run("idempotent per event and recipient", func(t *testing.T) {
		repo := newRepo(t)
		for _, n := range []domain.Notification{mk(1, Alice, 7), mk(2, Alice, 7), mk(3, Bob, 7)} {
			if err := repo.Add(ctx, n); err != nil {
				t.Fatal(err)
			}
		}
		a, _ := repo.ListFor(ctx, inbox(Alice), false, 50)
		b, _ := repo.ListFor(ctx, inbox(Bob), false, 50)
		if len(a) != 1 || len(b) != 1 || a[0].ID != nid(1) {
			t.Fatalf("alice=%v bob=%v", a, b)
		}
		if a[0].IssueKey != "KYB-1" || a[0].IssueTitle != "Login" || a[0].ActorName != "Carol" || a[0].Kind != domain.KindAssigned || !a[0].CreatedAt.Equal(t0.Add(time.Minute)) {
			t.Fatalf("round trip = %+v", a[0])
		}
	})

	t.Run("unread since, across workspaces (digest)", func(t *testing.T) {
		repo := newRepo(t)
		old, recent, read, other := mk(1, Alice, 1), mk(5, Alice, 2), mk(6, Alice, 3), mk(7, Bob, 4)
		for _, n := range []domain.Notification{old, recent, read, other} {
			if err := repo.Add(ctx, n); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.MarkRead(ctx, inbox(Alice), read.ID, t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, err := repo.UnreadSince(ctx, Alice, t0.Add(3*time.Minute), 50)
		if err != nil || len(got) != 1 || got[0].ID != recent.ID {
			t.Fatalf("UnreadSince = %v, %v", got, err)
		}
		if all, _ := repo.UnreadSince(ctx, Alice, time.Time{}, 50); len(all) != 2 || all[0].ID != recent.ID {
			t.Fatalf("newest first = %v", all)
		}
	})

	t.Run("announcements keep their project", func(t *testing.T) {
		repo := newRepo(t)
		n := mk(1, Alice, 9)
		n.Kind, n.IssueKey, n.Project, n.IssueTitle = domain.KindAnnouncement, "", "KYB", "Release Friday"
		if err := repo.Add(ctx, n); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.ListFor(ctx, inbox(Alice), false, 50)
		if len(got) != 1 || got[0].Kind != domain.KindAnnouncement || got[0].Project != "KYB" || got[0].IssueKey != "" {
			t.Fatalf("announcement = %+v", got)
		}
	})

	t.Run("doc comment keeps its page (S95)", func(t *testing.T) {
		repo := newRepo(t)
		n := mk(8, Alice, 12)
		n.Kind, n.IssueKey, n.Project, n.PageID = domain.KindDoc, "", "KYB", "82000000-0000-4000-8000-000000000001"
		if err := repo.Add(ctx, n); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.ListFor(ctx, inbox(Alice), false, 50); len(got) != 1 || got[0].PageID != n.PageID || got[0].Kind != domain.KindDoc {
			t.Fatalf("doc = %+v", got)
		}
	})

	t.Run("chat mention round trip", func(t *testing.T) {
		repo := newRepo(t)
		n := mk(5, Alice, 9)
		n.Kind, n.IssueKey, n.IssueTitle, n.ConversationID, n.ConversationName = domain.KindMentioned, "", "", "72000000-0000-4000-8000-000000000001", "general"
		if err := repo.Add(ctx, n); err != nil {
			t.Fatal(err)
		}
		a, _ := repo.ListFor(ctx, inbox(Alice), false, 50)
		if len(a) != 1 || a[0].ConversationID != n.ConversationID || a[0].ConversationName != "general" || a[0].IssueKey != "" {
			t.Fatalf("round trip = %+v", a)
		}
		// Reading the conversation reads its mentions only (S64).
		other := mk(6, Alice, 10)
		other.Kind, other.IssueKey, other.ConversationID = domain.KindMentioned, "", "72000000-0000-4000-8000-000000000002"
		_ = repo.Add(ctx, other)
		_ = repo.Add(ctx, mk(7, Alice, 11))
		if err := repo.MarkConversationRead(ctx, inbox(Alice), n.ConversationID, t0); err != nil {
			t.Fatal(err)
		}
		if c, _ := repo.UnreadCount(ctx, inbox(Alice)); c != 2 {
			t.Fatalf("unread after reading one conversation = %d", c)
		}
		if err := repo.MarkConversationRead(ctx, inbox(Alice), "not-a-uuid", t0); err != nil {
			t.Fatalf("malformed conversation = %v", err)
		}
	})

	t.Run("newest first, unread filter, counts and read state", func(t *testing.T) {
		repo := newRepo(t)
		for i := 1; i <= 3; i++ {
			_ = repo.Add(ctx, mk(i, Alice, int64(i)))
		}
		_ = repo.Add(ctx, mk(4, Bob, 4))
		list, _ := repo.ListFor(ctx, inbox(Alice), false, 2)
		if len(list) != 2 || list[0].ID != nid(3) || list[1].ID != nid(2) {
			t.Fatalf("newest first + limit = %v", list)
		}
		if c, _ := repo.UnreadCount(ctx, inbox(Alice)); c != 3 {
			t.Fatalf("unread = %d", c)
		}
		if err := repo.MarkRead(ctx, inbox(Alice), nid(2), t0); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkRead(ctx, inbox(Alice), nid(4), t0); !errors.Is(err, domain.ErrNotificationNotFound) {
			t.Fatalf("marking someone else's notification err = %v", err)
		}
		if err := repo.MarkRead(ctx, inbox(Alice), "not-a-uuid", t0); !errors.Is(err, domain.ErrNotificationNotFound) {
			t.Fatalf("malformed id err = %v", err)
		}
		unread, _ := repo.ListFor(ctx, inbox(Alice), true, 50)
		if len(unread) != 2 || unread[0].ID != nid(3) || unread[1].ID != nid(1) {
			t.Fatalf("unread list = %v", unread)
		}
		all, _ := repo.ListFor(ctx, inbox(Alice), false, 50)
		if !all[1].Read() || all[0].Read() {
			t.Fatalf("read flags = %v", all)
		}
		_ = repo.MarkAllRead(ctx, inbox(Alice), t0)
		if c, _ := repo.UnreadCount(ctx, inbox(Alice)); c != 0 {
			t.Fatalf("after read-all = %d", c)
		}
		if c, _ := repo.UnreadCount(ctx, inbox(Bob)); c != 1 {
			t.Fatalf("bob unaffected = %d", c)
		}
	})

	t.Run("inboxes are per workspace", func(t *testing.T) {
		repo := newRepo(t)
		here, there := mk(1, Alice, 1), mk(2, Alice, 2)
		there.Workspace = Other
		_ = repo.Add(ctx, here)
		_ = repo.Add(ctx, there)
		if l, _ := repo.ListFor(ctx, inbox(Alice), false, 50); len(l) != 1 || l[0].ID != here.ID || l[0].Workspace != WS {
			t.Fatalf("list = %+v", l)
		}
		other := domain.Inbox{Workspace: Other, User: Alice}
		if err := repo.MarkRead(ctx, inbox(Alice), there.ID, t0); !errors.Is(err, domain.ErrNotificationNotFound) {
			t.Fatalf("cross-workspace mark err = %v", err)
		}
		_ = repo.MarkAllRead(ctx, inbox(Alice), t0)
		if c, _ := repo.UnreadCount(ctx, other); c != 1 {
			t.Fatalf("other workspace unread = %d", c)
		}
		if l, err := repo.ListFor(ctx, domain.Inbox{Workspace: "bad", User: Alice}, false, 50); err != nil || len(l) != 0 {
			t.Fatalf("malformed inbox = %v %v", l, err)
		}
	})
}
