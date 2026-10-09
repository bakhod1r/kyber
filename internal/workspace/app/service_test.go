package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/workspace/adapter/memory"
	"github.com/bakhod1r/kyber/internal/workspace/app"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

func setup() *app.Service {
	n := 0
	return app.NewService(memory.NewRepository(), func() string { n++; return fmt.Sprintf("w-%d", n) })
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
