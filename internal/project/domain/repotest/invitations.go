// Package repotest is the contract every Project context repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

var at = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

// RunInvitations checks an InvitationRepository; ids must be UUIDs for SQL adapters, and ws a workspace
// that exists (the default one is always there).
func RunInvitations(t *testing.T, newRepo func(t *testing.T) domain.InvitationRepository, ids func(n int) domain.InvitationID) {
	ctx := tenant.With(context.Background(), string(domain.DefaultWorkspace))
	mk := func(t *testing.T, n int, project, email, hash string) *domain.Invitation {
		t.Helper()
		inv, err := domain.NewInvitation(ids(n), domain.DefaultWorkspace, project, email, domain.RoleMember, "30000000-0000-4000-8000-00000000000a", hash, at.Add(time.Duration(n)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		return inv
	}
	t.Run("save, find by token hash and id, list pending", func(t *testing.T) {
		r := newRepo(t)
		a, b, other := mk(t, 1, "KYB", "a@x.uz", "h1"), mk(t, 2, "KYB", "b@x.uz", "h2"), mk(t, 3, "OPS", "a@x.uz", "h3")
		for _, i := range []*domain.Invitation{a, b, other} {
			if err := r.Save(ctx, i); err != nil {
				t.Fatal(err)
			}
		}
		got, err := r.ByTokenHash(context.Background(), "h2") // the invitee is not in the workspace yet
		if err != nil || got.ID() != b.ID() || got.Email() != "b@x.uz" || got.Role() != domain.RoleMember ||
			!got.ExpiresAt().Equal(b.ExpiresAt()) || got.Workspace() != domain.DefaultWorkspace {
			t.Fatalf("ByTokenHash = %+v, %v", got, err)
		}
		if _, err := r.ByTokenHash(ctx, "nope"); !errors.Is(err, domain.ErrInvitationNotFound) {
			t.Fatalf("missing err = %v", err)
		}
		if got, err := r.ByID(ctx, a.ID()); err != nil || got.TokenHash() != "h1" {
			t.Fatalf("ByID = %+v, %v", got, err)
		}
		list, err := r.Pending(ctx, "KYB")
		if err != nil || len(list) != 2 || list[0].ID() != b.ID() || list[1].ID() != a.ID() {
			t.Fatalf("Pending = %v, %v", list, err)
		}
	})
	t.Run("accepting persists and leaves the pending list", func(t *testing.T) {
		r := newRepo(t)
		a := mk(t, 1, "KYB", "a@x.uz", "h1")
		_ = r.Save(ctx, a)
		if err := a.Accept("30000000-0000-4000-8000-00000000000b", "a@x.uz", at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if err := r.Save(ctx, a); err != nil {
			t.Fatal(err)
		}
		got, _ := r.ByID(ctx, a.ID())
		if got.AcceptedBy() != "30000000-0000-4000-8000-00000000000b" || !got.AcceptedAt().Equal(at.Add(time.Hour)) {
			t.Fatalf("accepted = %+v", got)
		}
		if list, _ := r.Pending(ctx, "KYB"); len(list) != 0 {
			t.Fatalf("Pending = %v", list)
		}
	})
	t.Run("a new invitation for the same address replaces the pending one", func(t *testing.T) {
		r := newRepo(t)
		_ = r.Save(ctx, mk(t, 1, "KYB", "a@x.uz", "h1"))
		again := mk(t, 2, "KYB", "A@x.uz", "h2")
		if err := r.Save(ctx, again); err != nil {
			t.Fatal(err)
		}
		if _, err := r.ByTokenHash(ctx, "h1"); !errors.Is(err, domain.ErrInvitationNotFound) {
			t.Fatalf("old link still works: %v", err)
		}
		if list, _ := r.Pending(ctx, "KYB"); len(list) != 1 || list[0].ID() != again.ID() {
			t.Fatalf("Pending = %v", list)
		}
	})
	t.Run("delete", func(t *testing.T) {
		r := newRepo(t)
		a := mk(t, 1, "KYB", "a@x.uz", "h1")
		_ = r.Save(ctx, a)
		if err := r.Delete(ctx, a.ID()); err != nil {
			t.Fatal(err)
		}
		if _, err := r.ByID(ctx, a.ID()); !errors.Is(err, domain.ErrInvitationNotFound) {
			t.Fatalf("after delete err = %v", err)
		}
	})
}
