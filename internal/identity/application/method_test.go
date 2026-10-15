package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
)

// S107c: a session remembers how its user signed in, so a workspace can require its SSO connection.
func TestSessionsRememberTheSignInMethod(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	_, _ = s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	pw, _ := s.Login(ctx, "ali@x.uz", "long enough pw", "127.0.0.1")
	sso, _, _ := s.LoginExternal(ctx, application.ExternalProfile{Provider: "sso:c-1", Subject: "okta-1", Email: "zara@acme.uz", EmailVerified: true, Name: "Zara"})
	google, _, _ := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-1", Email: "bob@gmail.com", EmailVerified: true, Name: "Bob"})
	for token, want := range map[string]string{pw: domain.MethodPassword, sso: "sso:c-1", google: "google"} {
		u, method, err := s.AuthenticateWithMethod(ctx, token)
		if err != nil || u == nil || method != want {
			t.Errorf("method = %q %v, want %q", method, err, want)
		}
	}
	if _, _, err := s.AuthenticateWithMethod(ctx, "nope"); err == nil {
		t.Fatal("an unknown token authenticated")
	}
}

func TestPersonalTokensAreATokenMethod(t *testing.T) {
	ctx := context.Background()
	s, _, _ := tokenService(plain)
	u, _ := s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	secret, _, _ := s.CreateToken(ctx, u.ID(), "MCP", 30)
	if _, method, err := s.AuthenticateWithMethod(ctx, secret); err != nil || method != domain.MethodToken {
		t.Fatalf("method = %q %v", method, err)
	}
}

// A token created while signed in through an SSO connection carries that sign-in, so it works in a
// workspace that requires the connection; any other token is a plain "token".
func TestTokensRememberTheSSOSignIn(t *testing.T) {
	ctx := context.Background()
	s, _, _ := tokenService(plain)
	u, _ := s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	viaSSO, _, _ := s.CreateTokenUnder(ctx, u.ID(), "cli", 30, "sso:c-1")
	viaPassword, _, _ := s.CreateTokenUnder(ctx, u.ID(), "cli", 30, domain.MethodPassword)
	for secret, want := range map[string]string{viaSSO: "sso:c-1", viaPassword: domain.MethodToken} {
		if _, method, err := s.AuthenticateWithMethod(ctx, secret); err != nil || method != want {
			t.Errorf("method = %q %v, want %q", method, err, want)
		}
	}
}

func TestOAuthTokensRememberTheConsentingSignIn(t *testing.T) {
	ctx := context.Background()
	c := &clock{now: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	n := 0
	s := application.NewService(repo, repo, &plainHasher{}, c, func() string { n++; return "id-" + string(rune('a'+n)) },
		application.WithPersonalTokens(repo), application.WithOAuth(repo))
	u, _ := s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	client, _ := s.RegisterClient(ctx, "203.0.113.1", "Claude", []string{"https://claude.ai/cb"})
	verifier := strings.Repeat("v", 50)
	code, err := s.Authorize(ctx, u.ID(), application.AuthorizeRequest{ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb",
		Challenge: s256(verifier), Method: "S256", SignInMethod: "sso:c-1"})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.ExchangeCode(ctx, application.TokenRequest{Code: code, ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: verifier})
	if err != nil {
		t.Fatal(err)
	}
	if _, method, err := s.AuthenticateWithMethod(ctx, token); err != nil || method != "sso:c-1" {
		t.Fatalf("method = %q %v", method, err)
	}
}
