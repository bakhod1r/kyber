package externaltest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// RunOAuth is the contract every OAuthStore adapter must pass; user must exist.
func RunOAuth(t *testing.T, s domain.OAuthStore, user domain.UserID) {
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	c, err := domain.NewOAuthClient("96000000-0000-4000-8000-000000000001", "Claude", []string{"https://claude.ai/cb", "http://localhost:3000/cb"}, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateClient(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.Client(ctx, c.ID)
	if err != nil || got.Name != "Claude" || len(got.RedirectURIs) != 2 || got.RedirectURIs[1] != "http://localhost:3000/cb" || !got.CreatedAt.Equal(at) {
		t.Fatalf("client = %+v %v", got, err)
	}
	for _, id := range []domain.ClientID{"96000000-0000-4000-8000-0000000000ff", "not-a-uuid"} {
		if _, err := s.Client(ctx, id); !errors.Is(err, domain.ErrOAuthClientNotFound) {
			t.Fatalf("missing client %s: %v", id, err)
		}
	}
	code, _ := domain.NewAuthCode([]byte("code-hash-0123456789abcdef0123456"), c.ID, user, "https://claude.ai/cb", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", at)
	code.Method = "sso:c-1"
	if err := s.SaveCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	taken, err := s.TakeCode(ctx, code.Hash)
	if err != nil || taken.Method != "sso:c-1" || taken.Client != c.ID || taken.User != user || taken.Challenge != code.Challenge || taken.RedirectURI != "https://claude.ai/cb" || !taken.ExpiresAt.Equal(code.ExpiresAt) {
		t.Fatalf("taken = %+v %v", taken, err)
	}
	if _, err := s.TakeCode(ctx, code.Hash); !errors.Is(err, domain.ErrAuthCodeNotFound) {
		t.Fatalf("a code is single-use: %v", err)
	}
}
