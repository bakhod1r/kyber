package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	app "github.com/bakhod1r/kyber/internal/chat/application"
	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
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

func setup(t *testing.T) (*app.Service, context.Context) {
	t.Helper()
	repo := memory.NewRepository()
	n := 0
	newID := func() string { n++; return fmt.Sprintf("id-%03d", n) }
	c := &clock{t: time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)}
	dir := members{"lead": {"w1"}, "dev": {"w1"}, "qa": {"w1"}, "eve": {"w2"}}
	return app.NewService(app.Deps{Conversations: repo, Messages: repo.Messages(), Members: dir, NewID: newID, Now: c.now}),
		tenant.With(context.Background(), "w1")
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
