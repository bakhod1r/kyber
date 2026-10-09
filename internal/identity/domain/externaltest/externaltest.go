// Package externaltest is the contract every ExternalIdentities adapter must pass.
package externaltest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// Run needs a store and a function creating a user (the link references it).
func Run(t *testing.T, newStore func(t *testing.T) (domain.ExternalIdentities, func(id domain.UserID))) {
	ctx := context.Background()
	s, mkUser := newStore(t)
	const u1, u2 = domain.UserID("40000000-0000-4000-8000-000000000001"), domain.UserID("40000000-0000-4000-8000-000000000002")
	mkUser(u1)
	mkUser(u2)
	if _, err := s.LinkedUser(ctx, domain.ProviderGoogle, "g-1"); !errors.Is(err, domain.ErrNotLinked) {
		t.Fatalf("unlinked err = %v", err)
	}
	for _, id := range []domain.ExternalIdentity{
		{Provider: domain.ProviderGoogle, Subject: "g-1", UserID: u1},
		{Provider: domain.ProviderTelegram, Subject: "g-1", UserID: u2}, // same subject, other provider
		{Provider: domain.ProviderTelegram, Subject: "42", UserID: u1},  // one user, several identities
	} {
		if err := s.Link(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		p    domain.Provider
		sub  string
		want domain.UserID
	}{{domain.ProviderGoogle, "g-1", u1}, {domain.ProviderTelegram, "g-1", u2}, {domain.ProviderTelegram, "42", u1}} {
		if got, err := s.LinkedUser(ctx, c.p, c.sub); err != nil || got != c.want {
			t.Fatalf("LinkedUser(%s, %s) = %s, %v", c.p, c.sub, got, err)
		}
	}
	// Re-linking the same identity is idempotent and never moves it to another user.
	if err := s.Link(ctx, domain.ExternalIdentity{Provider: domain.ProviderGoogle, Subject: "g-1", UserID: u2}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.LinkedUser(ctx, domain.ProviderGoogle, "g-1"); got != u1 {
		t.Fatalf("identity moved to %s", got)
	}
}
