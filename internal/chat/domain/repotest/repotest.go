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
	// Threads: replies stay out of history and unread, and are counted on the parent.
	m2 := posted[1]
	for i, at := range []int{20, 10} {
		r, _ := domain.NewReply(mid(10+i), m2, Dev, fmt.Sprintf("reply %d", i), t0.Add(time.Duration(at)*time.Minute))
		if err := msgs.Post(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	thread, err := msgs.Replies(ctx, m2.ID)
	if err != nil || len(thread) != 2 || thread[0].Body != "reply 1" || thread[0].Thread != m2.ID {
		t.Fatalf("replies = %v %v (oldest first)", thread, err)
	}
	top, _ := msgs.History(ctx, general.ID, "", 10)
	if len(top) != 5 || top[3].ID != m2.ID || top[3].ReplyCount != 2 || top[0].ReplyCount != 0 {
		t.Fatalf("history with threads = %+v", top)
	}
	if unread, _ := r.Unread(ctx, Lead); unread[general.ID] != 1 {
		t.Fatalf("lead unread = %v (Dev's msg 5 only; replies do not count)", unread)
	}
	if got, _ := msgs.ByID(ctx, mid(10)); got.Thread != m2.ID {
		t.Fatalf("reply ByID = %+v", got)
	}

	// Reactions are stored with the message.
	_, _ = m2.React(Dev, "🎉")
	_, _ = m2.React(Lead, "🎉")
	_, _ = m2.React(Lead, "+1")
	if err := msgs.Update(ctx, m2); err != nil {
		t.Fatal(err)
	}
	got2, _ := msgs.ByID(ctx, m2.ID)
	if len(got2.Reactions) != 2 || got2.Reactions[0].Emoji != "+1" || len(got2.Reactions[1].Users) != 2 || got2.Reactions[1].Users[0] != Lead {
		t.Fatalf("reactions = %+v", got2.Reactions)
	}
	if top, _ := msgs.History(ctx, general.ID, "", 10); len(top[3].Reactions) != 2 {
		t.Fatalf("history reactions = %+v", top[3])
	}
	_, _ = m2.React(Lead, "+1")
	_ = msgs.Update(ctx, m2)
	if got2, _ := msgs.ByID(ctx, m2.ID); len(got2.Reactions) != 1 {
		t.Fatalf("after toggle off = %+v", got2.Reactions)
	}

	// Pins: stored with the message; listed most recently pinned first.
	p1, _ := msgs.ByID(ctx, mid(1))
	p2, _ := msgs.ByID(ctx, mid(2))
	_ = p1.Pin(Dev, t0.Add(time.Hour))
	_ = p2.Pin(Lead, t0.Add(2*time.Hour))
	for _, m := range []*domain.Message{p1, p2} {
		if err := msgs.Update(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := msgs.Pinned(ctx, general.ID)
	if err != nil || len(pins) != 2 || pins[0].ID != p2.ID || pins[1].PinnedBy != Dev || !pins[1].PinnedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("pins = %+v %v", pins, err)
	}
	p2.Unpin()
	_ = msgs.Update(ctx, p2)
	if pins, _ := msgs.Pinned(ctx, general.ID); len(pins) != 1 {
		t.Fatalf("after unpin = %+v", pins)
	}

	// Search: every word, any case, live messages of the given conversations only, newest first.
	dmMsg, _ := domain.NewMessage(mid(30), dm.ID, Dev, "Deploy MSG to staging", t0.Add(3*time.Hour))
	_ = msgs.Post(ctx, dmMsg)
	found, err := msgs.Search(ctx, []domain.ConversationID{general.ID, dm.ID}, "msg", 10)
	if err != nil || len(found) != 5 || found[0].ID != dmMsg.ID {
		t.Fatalf("search msg = %v %v (msg 1, 2, msg three, 5 and the DM; msg 4 is deleted)", found, err)
	}
	if found, _ := msgs.Search(ctx, []domain.ConversationID{general.ID}, "staging", 10); len(found) != 0 {
		t.Fatalf("other conversations leak: %v", found)
	}
	if found, _ := msgs.Search(ctx, []domain.ConversationID{general.ID, dm.ID}, "  DEPLOY   staging ", 10); len(found) != 1 {
		t.Fatalf("all words = %v", found)
	}
	if found, _ := msgs.Search(ctx, []domain.ConversationID{general.ID}, "100%_", 10); len(found) != 0 {
		t.Fatalf("wildcards are literal: %v", found)
	}
	if found, _ := msgs.Search(ctx, []domain.ConversationID{general.ID, dm.ID}, "msg", 2); len(found) != 2 {
		t.Fatalf("limit = %v", found)
	}

	// Files: the attachment's name, type and size come back; deleting the message drops them.
	fm, _ := domain.NewFileMessage(mid(40), general.ID, Dev, domain.File{Name: "spec.pdf", Type: "application/pdf", Size: 1234}, t0.Add(4*time.Hour))
	if err := msgs.Post(ctx, fm); err != nil {
		t.Fatal(err)
	}
	if got, _ := msgs.ByID(ctx, fm.ID); got.File == nil || *got.File != (domain.File{Name: "spec.pdf", Type: "application/pdf", Size: 1234}) {
		t.Fatalf("file = %+v", got.File)
	}
	if got, _ := msgs.ByID(ctx, mid(1)); got.File != nil {
		t.Fatalf("plain message has a file: %+v", got.File)
	}
	_ = fm.Delete(Dev, t0.Add(5*time.Hour))
	_ = msgs.Update(ctx, fm)
	if got, _ := msgs.ByID(ctx, fm.ID); got.File != nil {
		t.Fatalf("deleted message keeps its file: %+v", got.File)
	}

	// System messages: stored with their kind and ref; the same ref only once.
	sys := domain.NewSystemMessage(mid(20), general.ID, Lead, domain.KindHuddleStarted, "call:k-1:started", "started a huddle", t0.Add(time.Hour))
	if err := msgs.Post(ctx, sys); err != nil {
		t.Fatal(err)
	}
	again := domain.NewSystemMessage(mid(21), general.ID, Lead, domain.KindHuddleStarted, "call:k-1:started", "started a huddle", t0.Add(time.Hour))
	if err := msgs.Post(ctx, again); !errors.Is(err, domain.ErrDuplicateRef) {
		t.Fatalf("duplicate ref = %v", err)
	}
	if got, err := msgs.ByID(ctx, sys.ID); err != nil || got.Kind != domain.KindHuddleStarted || got.Ref != "call:k-1:started" {
		t.Fatalf("system message = %+v %v", got, err)
	}
	if got, _ := msgs.ByID(ctx, mid(1)); got.Kind != "" || got.Ref != "" {
		t.Fatalf("people's message = %+v", got)
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

// RunSaved checks saved messages (S65) over the same store as r and msgs (users as for Run).
func RunSaved(t *testing.T, r domain.Repository, msgs domain.Messages, saved domain.SavedMessages) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	ch, _ := domain.NewChannel(cid(50), WS, "saved", "", false, Lead, t0)
	if err := r.Create(ctx, ch); err != nil {
		t.Fatal(err)
	}
	var posted []*domain.Message
	for i := 1; i <= 3; i++ {
		m, _ := domain.NewMessage(mid(50+i), ch.ID, Lead, fmt.Sprintf("message %d to remember", i), t0.Add(time.Duration(i)*time.Minute))
		if err := msgs.Post(ctx, m); err != nil {
			t.Fatal(err)
		}
		posted = append(posted, m)
	}
	put := func(s *domain.Saved) {
		t.Helper()
		if err := saved.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	put(domain.Save(WS, Dev, *posted[0], time.Time{}, t0))
	put(domain.Save(WS, Dev, *posted[1], t0.Add(time.Hour), t0.Add(time.Minute)))
	put(domain.Save(WS, Lead, *posted[2], t0.Add(time.Hour), t0))
	list, err := saved.For(ctx, WS, Dev)
	if err != nil || len(list) != 2 || list[0].Message != posted[1].ID || list[1].Message != posted[0].ID || !list[0].RemindAt.Equal(t0.Add(time.Hour)) ||
		list[0].Conversation != ch.ID || !list[1].RemindAt.IsZero() {
		t.Fatalf("Dev's saves = %+v %v (most recent first)", list, err)
	}

	due, err := saved.Due(ctx, t0.Add(time.Hour))
	if err != nil || len(due) != 2 {
		t.Fatalf("due = %+v %v", due, err)
	}
	for _, s := range due {
		if !s.Remind(t0.Add(time.Hour)) {
			t.Fatalf("not due: %+v", s)
		}
		if ev := s.PullEvents(); ev[0].Excerpt != fmt.Sprintf("message %d to remember", map[domain.MessageID]int{posted[1].ID: 2, posted[2].ID: 3}[s.Message]) {
			t.Fatalf("excerpt = %+v", ev)
		}
	}
	again, _ := saved.Due(ctx, t0.Add(time.Hour))
	again[0].Remind(t0.Add(time.Hour))
	if err := saved.MarkReminded(ctx, again[0]); err != nil {
		t.Fatal(err)
	}
	if err := saved.MarkReminded(ctx, again[0]); !errors.Is(err, domain.ErrAlreadyReminded) {
		t.Fatalf("second mark = %v", err)
	}
	if left, _ := saved.Due(ctx, t0.Add(time.Hour)); len(left) != 1 {
		t.Fatalf("due after one reminder = %+v", left)
	}

	// Saving again sets a new reminder; removing forgets the save.
	put(domain.Save(WS, again[0].User, *posted[map[domain.MessageID]int{posted[1].ID: 1, posted[2].ID: 2}[again[0].Message]], t0.Add(3*time.Hour), t0.Add(2*time.Hour)))
	if d, _ := saved.Due(ctx, t0.Add(3*time.Hour)); len(d) != 2 {
		t.Fatalf("due after saving again = %+v", d)
	}
	if err := saved.Remove(ctx, WS, Dev, posted[0].ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := saved.For(ctx, WS, Dev); len(l) != 1 {
		t.Fatalf("after remove = %+v", l)
	}
	if l, _ := saved.For(ctx, WS, Other); len(l) != 0 {
		t.Fatalf("Other's saves = %+v", l)
	}
}

// RunStatuses checks statuses (S66).
func RunStatuses(t *testing.T, st domain.Statuses) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	a, _ := domain.NewStatus(WS, Lead, "🏖", "On vacation", t0.Add(time.Hour), t0.Add(30*time.Minute), t0)
	b, _ := domain.NewStatus(WS, Dev, "", "", time.Time{}, time.Time{}, t0)
	for _, s := range []*domain.Status{a, b} {
		if err := st.Put(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.Of(ctx, WS, []domain.UserID{Lead, Other})
	if err != nil || len(got) != 1 || got[0].User != Lead || got[0].Emoji != "🏖" || got[0].Text != "On vacation" ||
		!got[0].ExpiresAt.Equal(t0.Add(time.Hour)) || !got[0].DNDUntil.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("Of = %+v %v", got, err)
	}
	// Setting again replaces; another workspace sees nothing.
	c, _ := domain.NewStatus(WS, Lead, "🎧", "Focus", time.Time{}, time.Time{}, t0)
	_ = st.Put(ctx, c)
	if got, _ := st.Of(ctx, WS, []domain.UserID{Lead}); len(got) != 1 || got[0].Text != "Focus" || !got[0].ExpiresAt.IsZero() || !got[0].DNDUntil.IsZero() {
		t.Fatalf("after replace = %+v", got)
	}
	if got, _ := st.Of(ctx, "00000000-0000-4000-8000-000000000009", []domain.UserID{Lead}); len(got) != 0 {
		t.Fatalf("other workspace = %+v", got)
	}
	if got, err := st.Of(ctx, WS, nil); err != nil || len(got) != 0 {
		t.Fatalf("no users = %+v %v", got, err)
	}
}

func sid(n int) domain.ScheduledID {
	return domain.ScheduledID(fmt.Sprintf("76000000-0000-4000-8000-%012d", n))
}

// RunScheduled checks scheduled messages and personal reminders (S67) over the store of r.
func RunScheduled(t *testing.T, r domain.Repository, sched domain.ScheduledMessages, rem domain.Reminders) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	ch, _ := domain.NewChannel(cid(60), WS, "sched", "", false, Lead, t0)
	if err := r.Create(ctx, ch); err != nil {
		t.Fatal(err)
	}
	a, _ := domain.NewScheduled(sid(1), WS, ch.ID, Lead, "later", t0.Add(2*time.Hour), t0)
	b, _ := domain.NewScheduled(sid(2), WS, ch.ID, Lead, "sooner", t0.Add(time.Hour), t0)
	c, _ := domain.NewScheduled(sid(3), WS, ch.ID, Dev, "dev's", t0.Add(time.Hour), t0)
	for _, s := range []*domain.Scheduled{a, b, c} {
		if err := sched.Add(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	mine, err := sched.Mine(ctx, WS, Lead)
	if err != nil || len(mine) != 2 || mine[0].ID != b.ID || mine[1].Body != "later" || !mine[1].SendAt.Equal(t0.Add(2*time.Hour)) ||
		mine[0].Conversation != ch.ID || mine[0].Workspace != WS {
		t.Fatalf("mine = %+v %v", mine, err)
	}
	_ = a.Reschedule("much later", t0.Add(3*time.Hour), t0)
	if err := sched.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	if got, err := sched.ByID(ctx, a.ID); err != nil || got.Body != "much later" || !got.SendAt.Equal(t0.Add(3*time.Hour)) || got.Author != Lead {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	if due, _ := sched.Due(ctx, t0.Add(time.Hour)); len(due) != 2 {
		t.Fatalf("due = %+v", due)
	}
	if err := sched.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.ScheduledID{b.ID, "nope"} {
		if _, err := sched.ByID(ctx, id); !errors.Is(err, domain.ErrScheduledNotFound) {
			t.Fatalf("deleted %s = %v", id, err)
		}
	}

	r1, _ := domain.NewReminder(domain.ReminderID(sid(10)), WS, Dev, ch.ID, "call the bank", t0.Add(time.Hour), t0)
	r2, _ := domain.NewReminder(domain.ReminderID(sid(11)), WS, Dev, ch.ID, "tomorrow", t0.Add(24*time.Hour), t0)
	for _, x := range []*domain.Reminder{r1, r2} {
		if err := rem.Add(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	due, err := rem.Due(ctx, t0.Add(time.Hour))
	if err != nil || len(due) != 1 || due[0].Text != "call the bank" || due[0].User != Dev || due[0].Workspace != WS {
		t.Fatalf("due reminders = %+v %v", due, err)
	}
	due[0].Remind(t0.Add(time.Hour))
	if err := rem.MarkReminded(ctx, due[0]); err != nil {
		t.Fatal(err)
	}
	if err := rem.MarkReminded(ctx, due[0]); !errors.Is(err, domain.ErrAlreadyReminded) {
		t.Fatalf("twice = %v", err)
	}
	if left, _ := rem.Due(ctx, t0.Add(time.Hour)); len(left) != 0 {
		t.Fatalf("left = %+v", left)
	}
}

// RunReceipts checks read cursors (S68).
func RunReceipts(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	dm, _ := domain.OpenDM(cid(70), WS, Lead, []domain.UserID{Dev}, t0)
	if err := r.Create(ctx, dm); err != nil {
		t.Fatal(err)
	}
	if got, err := r.ReadCursors(ctx, dm.ID); err != nil || len(got) != 0 {
		t.Fatalf("nobody read yet = %v %v", got, err)
	}
	_ = r.MarkRead(ctx, dm.ID, Dev, t0.Add(time.Minute))
	_ = r.MarkRead(ctx, dm.ID, Other, t0.Add(time.Minute)) // not a member: ignored
	got, err := r.ReadCursors(ctx, dm.ID)
	if err != nil || len(got) != 1 || !got[Dev].Equal(t0.Add(time.Minute)) {
		t.Fatalf("cursors = %v %v", got, err)
	}
}

// RunManagement checks saving a channel's name, topic, admins and archived state (S69).
func RunManagement(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	a, _ := domain.NewChannel(cid(80), WS, "alpha", "", false, Lead, t0)
	b, _ := domain.NewChannel(cid(81), WS, "beta", "", false, Lead, t0)
	for _, c := range []*domain.Conversation{a, b} {
		if err := r.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	_ = r.AddMember(ctx, a.ID, Dev, t0)
	a, _ = r.ByID(ctx, a.ID)
	_ = a.MakeAdmin(Lead, Dev)
	_ = a.Rename(Lead, "gamma", "Gamma team")
	_ = a.Archive(Lead, t0.Add(time.Hour))
	if err := r.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, _ := r.ByID(ctx, a.ID)
	if got.Name != "gamma" || got.Topic != "Gamma team" || len(got.Admins) != 1 || got.Admins[0] != Dev || !got.ArchivedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("after update = %+v", got)
	}
	got.ArchivedAt = time.Time{}
	got.Admins = nil
	_ = r.Update(ctx, got)
	if again, _ := r.ByID(ctx, a.ID); again.Archived() || len(again.Admins) != 0 {
		t.Fatalf("after unarchive = %+v", again)
	}
	b.Name = "gamma"
	if err := r.Update(ctx, b); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("taken name = %v", err)
	}
}
