package externaltest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// RunTokens is the contract every PersonalTokens adapter must pass; ann and ben must exist.
func RunTokens(t *testing.T, s domain.PersonalTokens, ann, ben domain.UserID) {
	ctx := context.Background()
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	exp := at.Add(24 * time.Hour)
	mk := func(id string, user domain.UserID, name, hash string, created time.Time, expires *time.Time) *domain.PersonalToken {
		tok, err := domain.NewPersonalToken(domain.TokenID(id), user, name, []byte(hash), created, expires)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.CreateToken(ctx, tok); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return tok
	}
	tok, _ := domain.NewPersonalToken("95000000-0000-4000-8000-000000000004", ann, "SSO", []byte("hash-sso-123456789abcdef012345678"), at, nil)
	tok.Method = "sso:c-1"
	if err := s.CreateToken(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if got, err := s.TokenByHash(ctx, tok.Hash); err != nil || got.Method != "sso:c-1" {
		t.Fatalf("method round trip = %+v %v", got, err)
	}
	if err := s.DeleteToken(ctx, ann, tok.ID); err != nil {
		t.Fatal(err)
	}
	ci := mk("95000000-0000-4000-8000-000000000001", ann, "CI", "hash-ci-0123456789abcdef01234567", at, &exp)
	mk("95000000-0000-4000-8000-000000000002", ann, "MCP", "hash-mcp-123456789abcdef012345678", at.Add(time.Minute), nil)
	mk("95000000-0000-4000-8000-000000000003", ben, "Ben's", "hash-ben-123456789abcdef012345678", at, nil)

	got, err := s.TokenByHash(ctx, []byte("hash-ci-0123456789abcdef01234567"))
	if err != nil || got.ID != ci.ID || got.UserID != ann || got.Name != "CI" || !got.ExpiresAt.Equal(exp) || got.LastUsedAt != nil || !got.CreatedAt.Equal(at) {
		t.Fatalf("by hash = %+v, %v", got, err)
	}
	if _, err := s.TokenByHash(ctx, []byte("nope")); !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("unknown hash: %v", err)
	}
	list, err := s.TokensOf(ctx, ann)
	if err != nil || len(list) != 2 || list[0].Name != "MCP" || list[1].Name != "CI" || list[0].ExpiresAt != nil {
		t.Fatalf("tokens of ann = %v, %v", list, err)
	}

	used := at.Add(time.Hour)
	if err := s.TouchToken(ctx, ci.ID, used); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.TokenByHash(ctx, ci.Hash); got.LastUsedAt == nil || !got.LastUsedAt.Equal(used) {
		t.Fatalf("touched = %+v", got)
	}

	if err := s.DeleteToken(ctx, ben, ci.ID); !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("ben revoked ann's token: %v", err)
	}
	if err := s.DeleteToken(ctx, ann, ci.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TokenByHash(ctx, ci.Hash); !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("revoked token still found: %v", err)
	}
	if err := s.DeleteToken(ctx, ann, "not-a-uuid"); !errors.Is(err, domain.ErrTokenNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}
