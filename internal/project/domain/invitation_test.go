package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

var inviteAt = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

func newInvitation(t *testing.T) *domain.Invitation {
	t.Helper()
	inv, err := domain.NewInvitation("i-1", domain.DefaultWorkspace, "KYB", "  Bob@X.uz ", domain.RoleMember, "u-alice", "hash", inviteAt)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestNewInvitation(t *testing.T) {
	inv := newInvitation(t)
	if inv.Email() != "bob@x.uz" || inv.Role() != domain.RoleMember || inv.Project() != "KYB" || inv.InvitedBy() != "u-alice" {
		t.Fatalf("invitation = %+v", inv)
	}
	if !inv.ExpiresAt().Equal(inviteAt.Add(domain.InvitationTTL)) || !inv.Pending(inviteAt) {
		t.Fatalf("expires %v, pending %v", inv.ExpiresAt(), inv.Pending(inviteAt))
	}
	for _, bad := range []string{"", "not-an-email", "x <a@b.uz>"} {
		if _, err := domain.NewInvitation("i-2", domain.DefaultWorkspace, "KYB", bad, domain.RoleMember, "u", "h", inviteAt); !errors.Is(err, domain.ErrInvalidEmail) {
			t.Errorf("email %q err = %v", bad, err)
		}
	}
	if _, err := domain.NewInvitation("i-2", domain.DefaultWorkspace, "KYB", "a@b.uz", "owner", "u", "h", inviteAt); !errors.Is(err, domain.ErrInvalidRole) {
		t.Errorf("role err = %v", err)
	}
}

func TestAcceptInvitation(t *testing.T) {
	inv := newInvitation(t)
	// Only the invited address may accept: a forwarded link does not let someone else join.
	if err := inv.Accept("u-eve", "eve@x.uz", inviteAt.Add(time.Hour)); !errors.Is(err, domain.ErrInvitationForSomeoneElse) {
		t.Fatalf("other email err = %v", err)
	}
	if err := inv.Accept("u-bob", "BOB@x.uz", inviteAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if inv.AcceptedBy() != "u-bob" || inv.Pending(inviteAt.Add(time.Hour)) {
		t.Fatalf("after accept: %+v", inv)
	}
	if err := inv.Accept("u-bob", "bob@x.uz", inviteAt.Add(2*time.Hour)); !errors.Is(err, domain.ErrInvitationUsed) {
		t.Fatalf("second accept err = %v", err)
	}
}

func TestExpiredInvitation(t *testing.T) {
	inv := newInvitation(t)
	late := inviteAt.Add(domain.InvitationTTL + time.Second)
	if inv.Pending(late) {
		t.Fatal("expired invitation pending")
	}
	if err := inv.Accept("u-bob", "bob@x.uz", late); !errors.Is(err, domain.ErrInvitationExpired) {
		t.Fatalf("expired accept err = %v", err)
	}
}
