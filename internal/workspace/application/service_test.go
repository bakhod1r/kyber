package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/workspace/application"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
	"github.com/bakhod1r/kyber/internal/workspace/infrastructure/memory"
)

func setup() *application.Service {
	n := 0
	return application.NewService(memory.NewRepository(), func() string { n++; return fmt.Sprintf("w-%d", n) })
}

func TestCreateAndMine(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, err := s.Create(ctx, "u-1", " Acme ", "Acme Corp")
	if err != nil || w.Slug() != "acme" || w.ID() != "w-1" {
		t.Fatalf("create = %+v %v", w, err)
	}
	_, _ = s.Create(ctx, "u-2", "globex", "Globex")
	if _, err := s.Create(ctx, "u-2", "acme", "Again"); !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("dup err = %v", err)
	}
	if _, err := s.Create(ctx, "u-2", "www", "W"); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("reserved err = %v", err)
	}
	mine, err := s.Mine(ctx, "u-1")
	if err != nil || len(mine) != 1 || mine[0].Slug() != "acme" {
		t.Fatalf("mine = %v %v", mine, err)
	}
	if got, err := s.Resolve(ctx, "acme"); err != nil || got.ID() != w.ID() {
		t.Fatalf("resolve = %+v %v", got, err)
	}
	if _, err := s.Resolve(ctx, "nope"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("resolve missing err = %v", err)
	}
}

func TestJoinAndMembership(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, _ := s.Create(ctx, "u-1", "acme", "Acme")
	if ok, _ := s.IsMember(ctx, string(w.ID()), "u-2"); ok {
		t.Fatal("not yet a member")
	}
	if err := s.Join(ctx, string(w.ID()), "u-2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Join(ctx, string(w.ID()), "u-1"); err != nil {
		t.Fatal(err) // joining again keeps the owner an owner
	}
	got, _ := s.Resolve(ctx, "acme")
	if r, _ := got.RoleOf("u-1"); r != domain.RoleOwner {
		t.Fatalf("owner demoted to %s", r)
	}
	if ok, err := s.IsMember(ctx, string(w.ID()), "u-2"); !ok || err != nil {
		t.Fatalf("member = %v %v", ok, err)
	}
	// The default workspace is open to every signed-in user (single-tenant mode).
	if ok, _ := s.IsMember(ctx, string(domain.DefaultID), "anyone"); !ok {
		t.Fatal("default workspace is open")
	}
	if err := s.Join(ctx, string(domain.DefaultID), "anyone"); err != nil {
		t.Fatal(err)
	}
	if err := s.Join(ctx, "missing", "u-1"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("join missing err = %v", err)
	}
	if _, err := s.IsMember(ctx, "missing", "u-1"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("member missing err = %v", err)
	}
}

func TestAdministers(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, _ := s.Create(ctx, "u-1", "acme", "Acme")
	_ = s.Join(ctx, string(w.ID()), "u-2")
	for _, tc := range []struct {
		ws, user string
		want     bool
	}{
		{string(w.ID()), "u-1", true},  // owner
		{string(w.ID()), "u-2", false}, // member
		{string(w.ID()), "u-9", false}, // stranger
		{string(domain.DefaultID), "u-1", false},
	} {
		if got, err := s.Administers(ctx, tc.ws, tc.user); err != nil || got != tc.want {
			t.Errorf("%s/%s = %v %v", tc.ws, tc.user, got, err)
		}
	}
	if _, err := s.Administers(ctx, "missing", "u-1"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestMemberIDs(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, _ := s.Create(ctx, "u-1", "acme", "Acme")
	_ = s.Join(ctx, string(w.ID()), "u-2")
	if ids, err := s.MemberIDs(ctx, string(w.ID())); err != nil || len(ids) != 2 || ids[0] != "u-1" || ids[1] != "u-2" {
		t.Fatalf("members = %v %v", ids, err)
	}
	if ids, err := s.MemberIDs(ctx, string(domain.DefaultID)); err != nil || ids != nil {
		t.Fatalf("default = %v %v", ids, err)
	}
	if _, err := s.MemberIDs(ctx, "missing"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestMembersAndRoles(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, _ := s.Create(ctx, "u-1", "acme", "Acme")
	ws := string(w.ID())
	_ = s.Join(ctx, ws, "u-2")
	if err := s.ChangeRole(ctx, ws, "u-2", "admin"); err != nil {
		t.Fatal(err)
	}
	ms, err := s.Members(ctx, ws)
	if err != nil || len(ms) != 2 || ms[1].UserID != "u-2" || ms[1].Role != domain.RoleAdmin {
		t.Fatalf("members = %+v %v", ms, err)
	}
	for _, tc := range []struct {
		ws, user, role string
		want           error
	}{
		{ws, "u-1", "member", domain.ErrLastOwner},
		{ws, "u-9", "member", domain.ErrNotMember},
		{ws, "u-2", "boss", domain.ErrInvalidRole},
		{"missing", "u-2", "member", domain.ErrWorkspaceNotFound},
	} {
		if err := s.ChangeRole(ctx, tc.ws, tc.user, tc.role); !errors.Is(err, tc.want) {
			t.Errorf("%+v: %v", tc, err)
		}
	}
	if ms, err := s.Members(ctx, string(domain.DefaultID)); err != nil || ms != nil {
		t.Fatalf("default = %v %v", ms, err)
	}
	if _, err := s.Members(ctx, "missing"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

// S106: SSO group sync grants and revokes workspace roles without an acting user.
func TestGrantAndRevoke(t *testing.T) {
	ctx := context.Background()
	s := setup()
	w, _ := s.Create(ctx, "u-owner", "acme", "Acme")
	ws := string(w.ID())
	if err := s.Grant(ctx, ws, "u-dev", "admin"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Administers(ctx, ws, "u-dev"); !ok {
		t.Fatal("not granted")
	}
	if err := s.Grant(ctx, ws, "u-dev", "member"); err != nil { // a change of role
		t.Fatal(err)
	}
	if ok, _ := s.Administers(ctx, ws, "u-dev"); ok {
		t.Fatal("role not changed")
	}
	if err := s.Revoke(ctx, ws, "u-dev"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.IsMember(ctx, ws, "u-dev"); ok {
		t.Fatal("still a member")
	}
	if err := s.Revoke(ctx, ws, "u-owner"); !errors.Is(err, domain.ErrLastOwner) {
		t.Fatalf("last owner: %v", err)
	}
	if err := s.Grant(ctx, ws, "u-x", "emperor"); err == nil {
		t.Fatal("invalid role accepted")
	}
	if err := s.Grant(ctx, "w-404", "u-x", "member"); !errors.Is(err, domain.ErrWorkspaceNotFound) {
		t.Fatalf("unknown workspace: %v", err)
	}
}
