package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/project/application"
	"github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/project/infrastructure/memory"
)

type sentMail struct{ workspace, to, subject, body string }

type fakeMailer struct {
	sent []sentMail
	err  error
}

func (m *fakeMailer) SendInvitation(_ context.Context, ws, to, subject, body, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, sentMail{ws, to, subject, body})
	return nil
}

type inviteFixture struct {
	s     *application.Service
	dir   fakeDirectory
	mail  *fakeMailer
	joins *joins
	now   time.Time
}

func newInviteFixture(t *testing.T) *inviteFixture {
	t.Helper()
	n, tok := 0, 0
	f := &inviteFixture{
		dir: fakeDirectory{
			"alice@x.uz": {ID: alice, Email: "alice@x.uz", Name: "Alice"},
			"bob@x.uz":   {ID: bob, Email: "bob@x.uz", Name: "Bob"},
		},
		mail: &fakeMailer{}, joins: &joins{}, now: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
	}
	f.s = application.NewService(memory.NewRepository(), f.dir, func() string { n++; return fmt.Sprintf("p-%d", n) }).
		WithWorkspaces(f.joins).
		WithInvitations(application.InvitationDeps{Repo: memory.NewInvitations(), Mailer: f.mail, PublicURL: "https://kyber.test",
			NewToken: func() string { tok++; return fmt.Sprintf("token-%d", tok) }, Now: func() time.Time { return f.now }})
	if _, err := f.s.Create(context.Background(), alice, "KYB", "Kyber"); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestInviteSomeoneWithoutAnAccount(t *testing.T) {
	ctx := context.Background()
	f := newInviteFixture(t)
	res, err := f.s.Invite(ctx, alice, "KYB", "Dan@X.uz", "member")
	if err != nil {
		t.Fatal(err)
	}
	if res.Added || res.Invitation == nil || res.Invitation.Email() != "dan@x.uz" || res.Link != "https://kyber.test/invite/token-1" || !res.Emailed {
		t.Fatalf("Invite = %+v", res)
	}
	if res.Invitation.TokenHash() == "token-1" || res.Invitation.TokenHash() == "" {
		t.Fatal("the token must be stored hashed")
	}
	if len(f.mail.sent) != 1 || f.mail.sent[0].to != "dan@x.uz" || !strings.Contains(f.mail.sent[0].body, res.Link) ||
		!strings.Contains(f.mail.sent[0].subject, "Kyber") || !strings.Contains(f.mail.sent[0].body, "Alice") {
		t.Fatalf("mail = %+v", f.mail.sent)
	}
	list, err := f.s.Invitations(ctx, alice, "KYB")
	if err != nil || len(list) != 1 {
		t.Fatalf("Invitations = %v, %v", list, err)
	}
	// Dan signs up with the invited address and accepts.
	f.dir["dan@x.uz"] = application.UserInfo{ID: "u-dan", Email: "dan@x.uz", Name: "Dan"}
	preview, err := f.s.InvitationByToken(ctx, "token-1")
	if err != nil || preview.Project != "KYB" || preview.ProjectName != "Kyber" || preview.InvitedBy != "Alice" || preview.Email != "dan@x.uz" {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	key, err := f.s.AcceptInvitation(ctx, "u-dan", "token-1")
	if err != nil || key != "KYB" {
		t.Fatalf("Accept = %q, %v", key, err)
	}
	if role, ok, _ := f.s.RoleOf(ctx, "KYB", "u-dan"); !ok || role != "member" {
		t.Fatalf("role = %q %v", role, ok)
	}
	if len(*f.joins) != 1 || (*f.joins)[0] != string(domain.DefaultWorkspace)+"/u-dan" {
		t.Fatalf("workspace joins = %v", *f.joins)
	}
	if _, err := f.s.AcceptInvitation(ctx, "u-dan", "token-1"); !errors.Is(err, domain.ErrInvitationUsed) {
		t.Fatalf("reuse err = %v", err)
	}
	if list, _ := f.s.Invitations(ctx, alice, "KYB"); len(list) != 0 {
		t.Fatalf("pending after accept = %v", list)
	}
}

func TestInviteAnExistingUserAddsThemRightAway(t *testing.T) {
	f := newInviteFixture(t)
	res, err := f.s.Invite(context.Background(), alice, "KYB", "bob@x.uz", "viewer")
	if err != nil || !res.Added || res.Invitation != nil || len(f.mail.sent) != 0 {
		t.Fatalf("Invite = %+v, %v (mail %v)", res, err, f.mail.sent)
	}
	if role, ok, _ := f.s.RoleOf(context.Background(), "KYB", string(bob)); !ok || role != "viewer" {
		t.Fatalf("role = %q %v", role, ok)
	}
}

func TestInvitationGuards(t *testing.T) {
	ctx := context.Background()
	f := newInviteFixture(t)
	_ = f.s.SetMember(ctx, alice, "KYB", "bob@x.uz", "member")
	if _, err := f.s.Invite(ctx, bob, "KYB", "dan@x.uz", "member"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member invites err = %v", err)
	}
	if _, err := f.s.Invitations(ctx, bob, "KYB"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member lists err = %v", err)
	}
	if _, err := f.s.Invite(ctx, alice, "KYB", "dan@x.uz", "owner"); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("role err = %v", err)
	}
	if _, err := f.s.Invite(ctx, alice, "KYB", "nope", "member"); !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("email err = %v", err)
	}
	res, _ := f.s.Invite(ctx, alice, "KYB", "dan@x.uz", "member")

	// A forwarded link does not let someone else in.
	if _, err := f.s.AcceptInvitation(ctx, bob, "token-1"); !errors.Is(err, domain.ErrInvitationForSomeoneElse) {
		t.Fatalf("other user err = %v", err)
	}
	if _, err := f.s.AcceptInvitation(ctx, bob, "forged"); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Fatalf("forged err = %v", err)
	}
	// Revoked links stop working.
	if err := f.s.RevokeInvitation(ctx, bob, "KYB", string(res.Invitation.ID())); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member revoke err = %v", err)
	}
	if err := f.s.RevokeInvitation(ctx, alice, "KYB", string(res.Invitation.ID())); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.InvitationByToken(ctx, "token-1"); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Fatalf("revoked err = %v", err)
	}
	// Expired links stop working.
	_, _ = f.s.Invite(ctx, alice, "KYB", "erin@x.uz", "member")
	f.dir["erin@x.uz"] = application.UserInfo{ID: "u-erin", Email: "erin@x.uz", Name: "Erin"}
	f.now = f.now.Add(domain.InvitationTTL + time.Minute)
	if _, err := f.s.AcceptInvitation(ctx, "u-erin", "token-2"); !errors.Is(err, domain.ErrInvitationExpired) {
		t.Fatalf("expired err = %v", err)
	}
}

// Without a mail server the invitation still works: the admin gets the link to share themselves.
func TestInviteWithoutMailer(t *testing.T) {
	f := newInviteFixture(t)
	f.mail.err = application.ErrNoMailer
	res, err := f.s.Invite(context.Background(), alice, "KYB", "dan@x.uz", "member")
	if err != nil || res.Emailed || res.Link == "" {
		t.Fatalf("Invite = %+v, %v", res, err)
	}
}
