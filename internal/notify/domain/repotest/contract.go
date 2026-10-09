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

func nid(n int) domain.NotificationID {
	return domain.NotificationID("91000000-0000-4000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

func Run(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	mk := func(n int, to string, event int64) domain.Notification {
		return domain.Notification{ID: nid(n), Recipient: domain.UserID(to), Kind: domain.KindAssigned, IssueKey: "KYB-1",
			IssueTitle: "Login", ActorName: "Carol", Excerpt: "x", SourceEvent: event, CreatedAt: t0.Add(time.Duration(n) * time.Minute)}
	}

	t.Run("idempotent per event and recipient", func(t *testing.T) {
		repo := newRepo(t)
		for _, n := range []domain.Notification{mk(1, Alice, 7), mk(2, Alice, 7), mk(3, Bob, 7)} {
			if err := repo.Add(ctx, n); err != nil {
				t.Fatal(err)
			}
		}
		a, _ := repo.ListFor(ctx, Alice, false, 50)
		b, _ := repo.ListFor(ctx, Bob, false, 50)
		if len(a) != 1 || len(b) != 1 || a[0].ID != nid(1) {
			t.Fatalf("alice=%v bob=%v", a, b)
		}
		if a[0].IssueKey != "KYB-1" || a[0].IssueTitle != "Login" || a[0].ActorName != "Carol" || a[0].Kind != domain.KindAssigned || !a[0].CreatedAt.Equal(t0.Add(time.Minute)) {
			t.Fatalf("round trip = %+v", a[0])
		}
	})

	t.Run("newest first, unread filter, counts and read state", func(t *testing.T) {
		repo := newRepo(t)
		for i := 1; i <= 3; i++ {
			_ = repo.Add(ctx, mk(i, Alice, int64(i)))
		}
		_ = repo.Add(ctx, mk(4, Bob, 4))
		list, _ := repo.ListFor(ctx, Alice, false, 2)
		if len(list) != 2 || list[0].ID != nid(3) || list[1].ID != nid(2) {
			t.Fatalf("newest first + limit = %v", list)
		}
		if c, _ := repo.UnreadCount(ctx, Alice); c != 3 {
			t.Fatalf("unread = %d", c)
		}
		if err := repo.MarkRead(ctx, Alice, nid(2), t0); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkRead(ctx, Alice, nid(4), t0); !errors.Is(err, domain.ErrNotificationNotFound) {
			t.Fatalf("marking someone else's notification err = %v", err)
		}
		if err := repo.MarkRead(ctx, Alice, "not-a-uuid", t0); !errors.Is(err, domain.ErrNotificationNotFound) {
			t.Fatalf("malformed id err = %v", err)
		}
		unread, _ := repo.ListFor(ctx, Alice, true, 50)
		if len(unread) != 2 || unread[0].ID != nid(3) || unread[1].ID != nid(1) {
			t.Fatalf("unread list = %v", unread)
		}
		all, _ := repo.ListFor(ctx, Alice, false, 50)
		if !all[1].Read() || all[0].Read() {
			t.Fatalf("read flags = %v", all)
		}
		_ = repo.MarkAllRead(ctx, Alice, t0)
		if c, _ := repo.UnreadCount(ctx, Alice); c != 0 {
			t.Fatalf("after read-all = %d", c)
		}
		if c, _ := repo.UnreadCount(ctx, Bob); c != 1 {
			t.Fatalf("bob unaffected = %d", c)
		}
	})
}
