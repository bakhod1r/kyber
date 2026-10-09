package app_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/adapter/memory"
	"github.com/bakhod1r/kyber/internal/notify/app"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type fakes struct {
	issues  map[string]app.IssueInfo
	members map[string]bool // "project/user"
	emails  map[string]string
	names   map[string]string
}

func (f *fakes) Lookup(_ context.Context, key string) (app.IssueInfo, error) {
	i, ok := f.issues[key]
	if !ok {
		return app.IssueInfo{}, app.ErrIssueGone
	}
	return i, nil
}
func (f *fakes) IsMember(_ context.Context, project, user string) (bool, error) {
	return f.members[project+"/"+user], nil
}
func (f *fakes) UserIDByEmail(_ context.Context, email string) (string, bool, error) {
	id, ok := f.emails[email]
	return id, ok, nil
}
func (f *fakes) DisplayName(_ context.Context, id string) (string, error) { return f.names[id], nil }

func setup() (*app.Service, *memory.Repository) {
	f := &fakes{
		issues:  map[string]app.IssueInfo{"KYB-1": {Project: "KYB", Title: "Login page", Reporter: "alice", Assignee: "bob"}},
		members: map[string]bool{"KYB/alice": true, "KYB/bob": true, "KYB/carol": true, "KYB/dave": true},
		emails:  map[string]string{"dave@x.uz": "dave", "eve@x.uz": "eve", "carol@x.uz": "carol"},
		names:   map[string]string{"alice": "Alice", "bob": "Bob", "carol": "Carol"},
	}
	repo := memory.NewRepository()
	n := 0
	return app.NewService(app.Deps{
		Repo: repo, Issues: f, Members: f, Users: f,
		NewID: func() string { n++; return fmt.Sprintf("n-%02d", n) },
		Now:   func() time.Time { return time.Date(2026, 4, 1, 10, n, 0, 0, time.UTC) },
	}), repo
}

func msg(id int64, name, payload string) outbox.Message {
	return outbox.Message{ID: id, Name: name, Payload: []byte(payload)}
}

func inbox(t *testing.T, s *app.Service, user string) string {
	t.Helper()
	items, _, err := s.List(context.Background(), user, false)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range items {
		out = append(out, string(n.Kind)+":"+n.IssueKey+":"+n.ActorName)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestAssigned(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob","by":"alice"}`)); err != nil {
		t.Fatal(err)
	}
	_ = s.OnIssueAssigned(ctx, msg(2, "issue.assigned", `{"key":"KYB-1","from":"bob","to":"alice","by":"alice"}`)) // self
	_ = s.OnIssueAssigned(ctx, msg(3, "issue.assigned", `{"key":"KYB-1","from":"alice","to":"","by":"alice"}`))    // unassign
	if got := inbox(t, s, "bob"); got != "assigned:KYB-1:Alice" {
		t.Fatalf("bob = %s", got)
	}
	if got := inbox(t, s, "alice"); got != "" {
		t.Fatalf("alice (self-assigned) = %s", got)
	}
	items, _, _ := s.List(ctx, "bob", false)
	if items[0].IssueTitle != "Login page" {
		t.Fatalf("title = %q", items[0].IssueTitle)
	}
}

func TestCommentAddedWithMentions(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	body := `{"issue_key":"KYB-1","author":"carol","body":"@dave@x.uz @eve@x.uz @bob@x.uz please check"}`
	if err := s.OnCommentAdded(ctx, msg(5, "comment.added", body)); err != nil {
		t.Fatal(err)
	}
	if got := inbox(t, s, "alice"); got != "commented:KYB-1:Carol" {
		t.Fatalf("reporter = %s", got)
	}
	if got := inbox(t, s, "bob"); got != "commented:KYB-1:Carol" {
		t.Fatalf("assignee (email unknown to directory, so not a mention) = %s", got)
	}
	if got := inbox(t, s, "dave"); got != "mentioned:KYB-1:Carol" {
		t.Fatalf("mentioned member = %s", got)
	}
	if got := inbox(t, s, "eve"); got != "" {
		t.Fatalf("non-member must never be notified: %s", got)
	}
	if got := inbox(t, s, "carol"); got != "" {
		t.Fatalf("author notified: %s", got)
	}
	items, _, _ := s.List(ctx, "dave", false)
	if items[0].Excerpt != "@dave@x.uz @eve@x.uz @bob@x.uz please check" {
		t.Fatalf("excerpt = %q", items[0].Excerpt)
	}
}

func TestRedeliveryIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	m := msg(9, "comment.added", `{"issue_key":"KYB-1","author":"carol","body":"hi"}`)
	for range 3 {
		if err := s.OnCommentAdded(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, unread, _ := s.List(ctx, "alice", false); unread != 1 {
		t.Fatalf("unread after 3 deliveries = %d", unread)
	}
}

func TestDeletedIssueAndBadPayload(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnCommentAdded(ctx, msg(1, "comment.added", `{"issue_key":"KYB-404","author":"carol","body":"x"}`)); err != nil {
		t.Fatalf("issue gone must be skipped, not retried forever: %v", err)
	}
	if err := s.OnIssueAssigned(ctx, msg(2, "issue.assigned", `not json`)); err != nil {
		t.Fatalf("malformed payload must be skipped (logged), not block the relay: %v", err)
	}
}

func TestReadState(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	_ = s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","to":"bob","by":"alice"}`))
	_ = s.OnCommentAdded(ctx, msg(2, "comment.added", `{"issue_key":"KYB-1","author":"carol","body":"x"}`))
	items, unread, _ := s.List(ctx, "bob", false)
	if len(items) != 2 || unread != 2 {
		t.Fatalf("items=%d unread=%d", len(items), unread)
	}
	if err := s.MarkRead(ctx, "alice", string(items[0].ID)); !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Fatalf("other user's notification err = %v", err)
	}
	_ = s.MarkRead(ctx, "bob", string(items[0].ID))
	if only, unread, _ := s.List(ctx, "bob", true); len(only) != 1 || unread != 1 {
		t.Fatalf("unread filter = %d / %d", len(only), unread)
	}
	_ = s.MarkAllRead(ctx, "bob")
	if _, unread, _ := s.List(ctx, "bob", false); unread != 0 {
		t.Fatalf("after read-all unread = %d", unread)
	}
}

// Regression (QA-06-1): issue.assigned events written before 0.6 have no "by";
// they must still be delivered (actor shown as "Someone"), never block the relay.
func TestLegacyAssignedEventWithoutActor(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	if err := s.OnIssueAssigned(ctx, msg(1, "issue.assigned", `{"key":"KYB-1","from":"","to":"bob"}`)); err != nil {
		t.Fatalf("legacy event err = %v", err)
	}
	if got := inbox(t, s, "bob"); got != "assigned:KYB-1:Someone" {
		t.Fatalf("bob = %s", got)
	}
}
