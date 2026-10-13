package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

func TestMemberReadModels(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	_, _ = s.Create(ctx, alice, "KYB", "Kyber")
	_, _ = s.Create(ctx, bob, "OPS", "Ops")
	if err := s.SetMember(ctx, alice, "KYB", "carol@x.uz", "member"); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.MemberIDs(ctx, "KYB"); err != nil || !slices.Equal(ids, []domain.UserID{alice, carol}) {
		t.Fatalf("MemberIDs = %v %v", ids, err)
	}
	if _, err := s.MemberIDs(ctx, "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("missing project: %v", err)
	}
	if ids, err := s.WorkspaceMemberIDs(ctx); err != nil || !slices.Equal(ids, []domain.UserID{alice, bob, carol}) {
		t.Fatalf("WorkspaceMemberIDs = %v %v", ids, err)
	}
	if ps, err := s.InWorkspace(ctx); err != nil || len(ps) != 2 || ps[0].Key() != "KYB" {
		t.Fatalf("InWorkspace = %v %v", ps, err)
	}
}
