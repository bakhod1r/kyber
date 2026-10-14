package externaltest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// DirectoryStore is a user store with the provisioning view.
type DirectoryStore interface {
	domain.Users
	domain.Sessions
	domain.Directory
}

// RunDirectory is the contract of every Directory adapter; ids must be valid user ids.
func RunDirectory(t *testing.T, s DirectoryStore, ids [3]domain.UserID) {
	ctx := context.Background()
	for i, e := range []domain.Email{"carol@dir.uz", "alice@dir.uz", "bob@dir.uz"} {
		u, _ := domain.NewUser(ids[i], e, "U", "")
		if err := s.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	page, total, err := s.ListUsers(ctx, "", 1, 1)
	if err != nil || total < 3 || len(page) != 1 {
		t.Fatalf("page = %v %d %v", page, total, err)
	}
	all, _, _ := s.ListUsers(ctx, "", 0, 100)
	var dirEmails []domain.Email
	for _, u := range all {
		if u.Email().String() == "alice@dir.uz" || u.Email().String() == "bob@dir.uz" || u.Email().String() == "carol@dir.uz" {
			dirEmails = append(dirEmails, u.Email())
		}
	}
	if len(dirEmails) != 3 || dirEmails[0] != "alice@dir.uz" || dirEmails[2] != "carol@dir.uz" {
		t.Fatalf("ordered by email = %v", dirEmails)
	}
	one, total, _ := s.ListUsers(ctx, "bob@dir.uz", 0, 10)
	if total != 1 || len(one) != 1 || one[0].ID() != ids[2] {
		t.Fatalf("filter = %v %d", one, total)
	}

	bob := one[0]
	_ = bob.Rename("Bob Builder")
	bob.Deactivate()
	if err := s.SaveUser(ctx, bob); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ByID(ctx, bob.ID())
	if got.Active() || got.Name() != "Bob Builder" {
		t.Fatalf("saved = %q active=%v", got.Name(), got.Active())
	}
	if byEmail, _ := s.ByEmail(ctx, "bob@dir.uz"); byEmail.Active() {
		t.Fatal("ByEmail must carry the active flag")
	}
	ghost, _ := domain.NewUser("40000000-0000-4000-8000-0000000000ff", "ghost@dir.uz", "G", "")
	if err := s.SaveUser(ctx, ghost); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("unknown user: %v", err)
	}

	_ = s.CreateSession(ctx, domain.Session{TokenHash: []byte("dir-session-hash-1"), UserID: bob.ID(), ExpiresAt: time.Now().Add(time.Hour)})
	if err := s.DeleteSessionsOf(ctx, bob.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByHash(ctx, []byte("dir-session-hash-1")); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("sessions survive: %v", err)
	}
}
