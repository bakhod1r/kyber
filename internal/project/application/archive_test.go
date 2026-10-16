package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

// B10: admins archive and restore a project; while archived it stays listed (marked) and readable, and its
// membership cannot change.
func TestArchiveAndRestore(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	_, _ = s.Create(ctx, alice, "KYB", "Kyber")
	_ = s.SetMember(ctx, alice, "KYB", "bob@x.uz", "member")
	if _, err := s.Archive(ctx, bob, "KYB"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member archives: %v", err)
	}
	p, err := s.Archive(ctx, alice, "KYB")
	if err != nil || !p.Archived() {
		t.Fatalf("Archive = %v, %v", p, err)
	}
	if archived, _ := s.Archived(ctx, "KYB"); !archived {
		t.Fatal("Archived = false")
	}
	if list, _ := s.List(ctx, bob); len(list) != 1 || !list[0].Archived() {
		t.Fatalf("List = %v", list)
	}
	if err := s.SetMember(ctx, alice, "KYB", "carol@x.uz", "viewer"); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("membership while archived: %v", err)
	}
	if _, err := s.Restore(ctx, bob, "KYB"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member restores: %v", err)
	}
	if p, err := s.Restore(ctx, alice, "KYB"); err != nil || p.Archived() {
		t.Fatalf("Restore = %v, %v", p, err)
	}
	if err := s.SetMember(ctx, alice, "KYB", "carol@x.uz", "viewer"); err != nil {
		t.Fatalf("membership after restore: %v", err)
	}
}
