// Package repotest is the contract every Chat repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

const (
	WS    = "00000000-0000-4000-8000-000000000001"
	Lead  = "70000000-0000-4000-8000-000000000001"
	Dev   = "70000000-0000-4000-8000-000000000002"
	Other = "70000000-0000-4000-8000-000000000003"
)

func cid(n int) domain.ConversationID {
	return domain.ConversationID(fmt.Sprintf("72000000-0000-4000-8000-%012d", n))
}
func mid(n int) domain.MessageID {
	return domain.MessageID(fmt.Sprintf("73000000-0000-4000-8000-%012d", n))
}

// Run needs repositories over one store that already has the users Lead, Dev and Other.
func Run(t *testing.T, r domain.Repository, msgs domain.Messages) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	general, _ := domain.NewChannel(cid(1), WS, "general", "All hands", false, Lead, t0)
	secret, _ := domain.NewChannel(cid(2), WS, "secret", "", true, Lead, t0)
	dm, _ := domain.OpenDM(cid(3), WS, Dev, []domain.UserID{Other}, t0)
	for _, c := range []*domain.Conversation{general, secret, dm} {
		if err := r.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	dup, _ := domain.NewChannel(cid(4), WS, "general", "", true, Dev, t0)
	if err := r.Create(ctx, dup); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("duplicate name = %v", err)
	}
	got, err := r.ByID(ctx, general.ID)
	if err != nil || got.Name != "general" || got.Topic != "All hands" || got.Private || got.Kind != domain.KindChannel ||
		got.CreatedBy != Lead || !got.CreatedAt.Equal(t0) || len(got.Members) != 1 || got.Workspace != WS {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	if got, err := r.DMByKey(ctx, WS, dm.Key); err != nil || got.ID != dm.ID || got.Kind != domain.KindDM || len(got.Members) != 2 {
		t.Fatalf("DMByKey = %+v %v", got, err)
	}
	for _, err := range []error{
		func() error { _, err := r.DMByKey(ctx, WS, "x,y"); return err }(),
		func() error { _, err := r.ByID(ctx, cid(9)); return err }(),
		func() error { _, err := r.ByID(ctx, "not-a-uuid"); return err }(),
	} {
		if !errors.Is(err, domain.ErrConversationNotFound) {
			t.Fatalf("missing = %v", err)
		}
	}

	// Membership: Dev joins general twice (no-op), and sees general + DM but not secret.
	for range 2 {
		if err := r.AddMember(ctx, general.ID, Dev, t0); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := r.ByID(ctx, general.ID); len(got.Members) != 2 || got.Members[0] != Lead || got.Members[1] != Dev {
		t.Fatalf("members = %v", got.Members)
	}
	vis, err := r.Visible(ctx, WS, Dev)
	if err != nil || len(vis) != 2 || vis[0].ID != dm.ID || vis[1].ID != general.ID {
		t.Fatalf("visible(dev) = %v %v (DM name is empty, so first)", vis, err)
	}
	if vis, _ := r.Visible(ctx, WS, Lead); len(vis) != 2 || vis[0].Name != "general" || vis[1].Name != "secret" {
		t.Fatalf("visible(lead) = %v", vis)
	}
	if vis, _ := r.Visible(ctx, "00000000-0000-4000-8000-000000000009", Lead); len(vis) != 0 {
		t.Fatalf("other workspace = %v", vis)
	}
	if err := r.RemoveMember(ctx, general.ID, Dev); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.ByID(ctx, general.ID); len(got.Members) != 1 {
		t.Fatalf("after remove = %v", got.Members)
	}
	_ = r.AddMember(ctx, general.ID, Dev, t0)

	// Messages and history paging, newest first.
	var posted []*domain.Message
	for i := 1; i <= 5; i++ {
		author := domain.UserID(Lead)
		if i == 5 {
			author = Dev
		}
		m, _ := domain.NewMessage(mid(i), general.ID, author, fmt.Sprintf("msg %d", i), t0.Add(time.Duration(i)*time.Minute))
		if err := msgs.Post(ctx, m); err != nil {
			t.Fatal(err)
		}
		posted = append(posted, m)
	}
	page, err := msgs.History(ctx, general.ID, "", 2)
	if err != nil || len(page) != 2 || page[0].Body != "msg 5" || page[1].Body != "msg 4" || page[0].Author != Dev {
		t.Fatalf("page 1 = %v %v", page, err)
	}
	page, _ = msgs.History(ctx, general.ID, page[1].ID, 10)
	if len(page) != 3 || page[0].Body != "msg 3" || page[2].Body != "msg 1" || !page[2].CreatedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("page 2 = %v", page)
	}
	if page, _ := msgs.History(ctx, dm.ID, "", 10); len(page) != 0 {
		t.Fatalf("empty history = %v", page)
	}

	// Unread: messages by others after the read cursor (deleted ones do not count).
	unread, err := r.Unread(ctx, Dev)
	if err != nil || unread[general.ID] != 4 || unread[dm.ID] != 0 {
		t.Fatalf("unread(dev) = %v %v", unread, err)
	}
	if err := r.MarkRead(ctx, general.ID, Dev, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	m4 := posted[3]
	_ = m4.Delete(Lead, t0.Add(time.Hour))
	if err := msgs.Update(ctx, m4); err != nil {
		t.Fatal(err)
	}
	if unread, _ := r.Unread(ctx, Dev); unread[general.ID] != 1 {
		t.Fatalf("unread after read + delete = %v", unread)
	}
	m3 := posted[2]
	_ = m3.Edit(Lead, "msg three", t0.Add(time.Hour))
	_ = msgs.Update(ctx, m3)
	if got, err := msgs.ByID(ctx, m3.ID); err != nil || got.Body != "msg three" || !got.EditedAt.Equal(t0.Add(time.Hour)) || got.Conversation != general.ID {
		t.Fatalf("edited = %+v %v", got, err)
	}
	if got, _ := msgs.ByID(ctx, m4.ID); !got.Deleted || got.Body != "" {
		t.Fatalf("deleted = %+v", got)
	}
	if got, _ := msgs.ByID(ctx, mid(1)); !got.EditedAt.IsZero() {
		t.Fatalf("never edited = %+v", got)
	}
	for _, err := range []error{
		func() error { _, err := msgs.ByID(ctx, mid(99)); return err }(),
		func() error { _, err := msgs.ByID(ctx, "nope"); return err }(),
	} {
		if !errors.Is(err, domain.ErrMessageNotFound) {
			t.Fatalf("missing message = %v", err)
		}
	}
}
