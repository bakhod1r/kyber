package application_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
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
