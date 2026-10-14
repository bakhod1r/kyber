package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
)

func TestSCIMProvisioning(t *testing.T) {
	ctx := context.Background()
	n := 0
	ids := func() string { n++; return "id-" + string(rune('a'+n)) }
	c := &clock{now: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	s := application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithPersonalTokens(repo), application.WithDirectory(repo))

	u, err := s.ProvisionUser(ctx, "Ann@X.uz", "Ann", true)
	if err != nil || u.Email() != "ann@x.uz" || u.PasswordHash() != "" || !u.Active() {
		t.Fatalf("provision = %+v %v", u, err)
	}
	if _, err := s.ProvisionUser(ctx, "ann@x.uz", "Again", true); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	list, total, err := s.DirectoryUsers(ctx, "ann@x.uz", 0, 10)
	if err != nil || total != 1 || list[0].ID() != u.ID() {
		t.Fatalf("list = %v %d %v", list, total, err)
	}

	// A provisioned user who also has a password, a session and a token loses all of them on deactivation.
	member, _ := s.Signup(ctx, application.Signup{Email: "bo@x.uz", Name: "Bo", Password: "long enough pw"})
	session, _ := s.Login(ctx, "bo@x.uz", "long enough pw", "1.2.3.4")
	pat, _, _ := s.CreateToken(ctx, member.ID(), "CI", 0)
	active := false
	if _, err := s.UpdateDirectoryUser(ctx, string(member.ID()), application.DirectoryChange{Active: &active}); err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{"session": session, "token": pat} {
		if _, err := s.Authenticate(ctx, tok); !errors.Is(err, application.ErrUnauthenticated) {
			t.Errorf("%s after deactivation: %v", name, err)
		}
	}
	if _, err := s.Login(ctx, "bo@x.uz", "long enough pw", "1.2.3.4"); !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("login when inactive: %v", err)
	}
	active, name := true, "Bo Renamed"
	got, err := s.UpdateDirectoryUser(ctx, string(member.ID()), application.DirectoryChange{Active: &active, Name: &name})
	if err != nil || !got.Active() || got.Name() != "Bo Renamed" {
		t.Fatalf("reactivate = %+v %v", got, err)
	}
	if _, err := s.Login(ctx, "bo@x.uz", "long enough pw", "1.2.3.4"); err != nil {
		t.Fatalf("login after reactivation: %v", err)
	}
	if _, err := s.UpdateDirectoryUser(ctx, "nope", application.DirectoryChange{}); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := s.DirectoryUser(ctx, string(u.ID())); err != nil {
		t.Fatal(err)
	}

	off := application.NewService(repo, repo, &plainHasher{}, c, ids)
	if _, err := off.ProvisionUser(ctx, "x@x.uz", "X", true); !errors.Is(err, application.ErrDirectoryDisabled) {
		t.Fatalf("disabled: %v", err)
	}
}
