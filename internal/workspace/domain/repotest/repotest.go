// Package repotest is the contract every workspace Repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

// Run needs a repository and a function creating users (members reference them).
func Run(t *testing.T, newRepo func(t *testing.T) (domain.Repository, func(domain.UserID))) {
	ctx := context.Background()
	r, mkUser := newRepo(t)
	const u1, u2 = domain.UserID("60000000-0000-4000-8000-000000000001"), domain.UserID("60000000-0000-4000-8000-000000000002")
	mkUser(u1)
	mkUser(u2)
	acme, _ := domain.NewWorkspace("61000000-0000-4000-8000-000000000001", "acme", "Acme", u1)
	globex, _ := domain.NewWorkspace("61000000-0000-4000-8000-000000000002", "globex", "Globex", u2)
	for _, w := range []*domain.Workspace{acme, globex} {
		if err := r.Create(ctx, w); err != nil {
			t.Fatal(err)
		}
	}
	dup, _ := domain.NewWorkspace("61000000-0000-4000-8000-000000000003", "acme", "Again", u2)
	if err := r.Create(ctx, dup); !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("dup err = %v", err)
	}
	got, err := r.BySlug(ctx, "acme")
	if err != nil || got.ID() != acme.ID() || got.Name() != "Acme" || !got.IsMember(u1) || got.IsMember(u2) {
		t.Fatalf("BySlug = %+v %v", got, err)
	}
	if got, err := r.ByID(ctx, globex.ID()); err != nil || got.Slug() != "globex" {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	for _, miss := range []func() error{
		func() error { _, err := r.BySlug(ctx, "nope"); return err },
		func() error { _, err := r.ByID(ctx, "61000000-0000-4000-8000-000000000009"); return err },
		func() error { _, err := r.ByID(ctx, "not-a-uuid"); return err },
	} {
		if err := miss(); !errors.Is(err, domain.ErrWorkspaceNotFound) {
			t.Fatalf("missing err = %v", err)
		}
	}
	_ = acme.AddMember(u2, domain.RoleMember)
	if err := r.SaveMembers(ctx, acme); err != nil {
		t.Fatal(err)
	}
	mine, err := r.ForUser(ctx, u2)
	if err != nil || len(mine) != 2 || mine[0].Slug() != "acme" || mine[1].Slug() != "globex" {
		t.Fatalf("ForUser = %v %v", mine, err)
	}
	if r2, _ := mine[0].RoleOf(u2); r2 != domain.RoleMember {
		t.Fatalf("role = %s", r2)
	}
	// AddMember is atomic: concurrent joins are all kept and existing roles are untouched.
	const u3, u4 = domain.UserID("60000000-0000-4000-8000-000000000003"), domain.UserID("60000000-0000-4000-8000-000000000004")
	mkUser(u3)
	mkUser(u4)
	var wg sync.WaitGroup
	for _, u := range []domain.UserID{u3, u4, u1} {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.AddMember(ctx, globex.ID(), u, domain.RoleMember) }()
	}
	wg.Wait()
	g, _ := r.ByID(ctx, globex.ID())
	if !g.IsMember(u3) || !g.IsMember(u4) || !g.IsMember(u1) {
		t.Fatalf("concurrent joins lost a member: %v", g.Members())
	}
	if role, _ := g.RoleOf(u2); role != domain.RoleOwner {
		t.Fatalf("owner role changed to %s", role)
	}
	if err := r.AddMember(ctx, globex.ID(), u2, domain.RoleMember); err != nil {
		t.Fatal(err)
	}
	if g, _ := r.ByID(ctx, globex.ID()); func() domain.Role { r, _ := g.RoleOf(u2); return r }() != domain.RoleOwner {
		t.Fatal("AddMember must not demote an existing member")
	}

	// SaveMembers replaces the member list (removals persist too).
	removed := domain.Rehydrate(acme.ID(), "acme", "Acme", []domain.Member{{UserID: u1, Role: domain.RoleOwner}})
	if err := r.SaveMembers(ctx, removed); err != nil {
		t.Fatal(err)
	}
	if mine, _ := r.ForUser(ctx, u2); len(mine) != 1 {
		t.Fatalf("after removal = %v", mine)
	}
}
