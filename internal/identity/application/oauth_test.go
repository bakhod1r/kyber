package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
)

func s256(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestOAuthAuthorizationCodeFlow(t *testing.T) {
	ctx := context.Background()
	n := 0
	ids := func() string { n++; return "id-" + string(rune('a'+n)) }
	c := &clock{now: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	s := application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithPersonalTokens(repo), application.WithOAuth(repo))
	u, _ := s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	client, err := s.RegisterClient(ctx, "203.0.113.1", "Claude", []string{"https://claude.ai/cb"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.OAuthClient(ctx, string(client.ID)); err != nil || got.Name != "Claude" {
		t.Fatalf("client = %+v %v", got, err)
	}
	verifier := strings.Repeat("v", 50)
	req := application.AuthorizeRequest{ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Challenge: s256(verifier), Method: "S256"}
	code, err := s.Authorize(ctx, u.ID(), req)
	if err != nil || len(code) < 40 {
		t.Fatalf("authorize = %q %v", code, err)
	}
	exchange := application.TokenRequest{Code: code, ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: verifier}
	token, ttl, err := s.ExchangeCode(ctx, exchange)
	if err != nil || !application.IsPersonalToken(token) || ttl != application.OAuthTokenDays*24*time.Hour {
		t.Fatalf("exchange = %q %v %v", token, ttl, err)
	}
	if got, err := s.Authenticate(ctx, token); err != nil || got.ID() != u.ID() {
		t.Fatalf("token authenticates its user: %v %v", got, err)
	}
	if list, _ := s.Tokens(ctx, u.ID()); len(list) != 1 || list[0].Name != "MCP: Claude" {
		t.Fatalf("token listed for revocation: %+v", list)
	}
	if _, _, err := s.ExchangeCode(ctx, exchange); !errors.Is(err, application.ErrInvalidGrant) {
		t.Fatalf("replayed code: %v", err)
	}

	fresh := func() string {
		code, err := s.Authorize(ctx, u.ID(), req)
		if err != nil {
			t.Fatal(err)
		}
		return code
	}
	bad := map[string]application.TokenRequest{
		"wrong verifier": {Code: fresh(), ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: strings.Repeat("w", 50)},
		"wrong redirect": {Code: fresh(), ClientID: string(client.ID), RedirectURI: "https://claude.ai/other", Verifier: verifier},
		"wrong client":   {Code: fresh(), ClientID: "someone-else", RedirectURI: "https://claude.ai/cb", Verifier: verifier},
		"unknown code":   {Code: "nope", ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: verifier},
	}
	for name, r := range bad {
		if _, _, err := s.ExchangeCode(ctx, r); !errors.Is(err, application.ErrInvalidGrant) {
			t.Errorf("%s: %v", name, err)
		}
	}
	late := fresh()
	c.now = c.now.Add(domain.AuthCodeTTL)
	if _, _, err := s.ExchangeCode(ctx, application.TokenRequest{Code: late, ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: verifier}); !errors.Is(err, application.ErrInvalidGrant) {
		t.Fatalf("expired code: %v", err)
	}

	for name, r := range map[string]application.AuthorizeRequest{
		"unknown client": {ClientID: "nope", RedirectURI: "https://claude.ai/cb", Challenge: s256(verifier), Method: "S256"},
		"bad redirect":   {ClientID: string(client.ID), RedirectURI: "https://evil.example/cb", Challenge: s256(verifier), Method: "S256"},
		"plain PKCE":     {ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Challenge: s256(verifier), Method: "plain"},
		"no challenge":   {ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Method: "S256"},
	} {
		if _, err := s.Authorize(ctx, u.ID(), r); err == nil {
			t.Errorf("%s: authorized", name)
		}
	}
	if _, err := s.RegisterClient(ctx, "203.0.113.1", "Evil", []string{"http://evil.example/cb"}); !errors.Is(err, domain.ErrInvalidOAuthClient) {
		t.Fatalf("plain http redirect: %v", err)
	}

	// A user deactivated (SCIM) between consent and exchange gets no token.
	pending := fresh()
	u.Deactivate()
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ExchangeCode(ctx, application.TokenRequest{Code: pending, ClientID: string(client.ID), RedirectURI: "https://claude.ai/cb", Verifier: verifier}); !errors.Is(err, application.ErrInvalidGrant) {
		t.Fatalf("inactive user: %v", err)
	}

	off := application.NewService(repo, repo, &plainHasher{}, c, ids)
	if _, err := off.RegisterClient(ctx, "203.0.113.1", "X", []string{"https://x.example/cb"}); !errors.Is(err, application.ErrOAuthDisabled) {
		t.Fatalf("disabled: %v", err)
	}
}

// Anonymous registration is limited per client IP (ADR-0016): the shared LoginAttempts per LoginWindow.
func TestRegisterClientIsRateLimitedPerIP(t *testing.T) {
	ctx := context.Background()
	n := 0
	ids := func() string { n++; return "id-" + strings.Repeat("x", n) }
	c := &clock{now: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	s := application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithOAuth(repo))
	for i := range application.LoginAttempts {
		if _, err := s.RegisterClient(ctx, "198.51.100.7", "Bot", []string{"https://bot.example/cb"}); err != nil {
			t.Fatalf("registration %d: %v", i+1, err)
		}
	}
	_, err := s.RegisterClient(ctx, "198.51.100.7", "Bot", []string{"https://bot.example/cb"})
	var tooMany *application.TooManyAttemptsError
	if !errors.As(err, &tooMany) || tooMany.RetryAfter <= 0 {
		t.Fatalf("over the limit: %v", err)
	}
	if _, err := s.RegisterClient(ctx, "198.51.100.8", "Bot", []string{"https://bot.example/cb"}); err != nil {
		t.Fatalf("another IP: %v", err)
	}
	// Registrations and logins from the same IP are counted separately.
	if _, err := s.Login(ctx, "nobody@x.uz", "whatever pw", "198.51.100.7"); errors.As(err, &tooMany) {
		t.Fatalf("login throttled by registrations: %v", err)
	}

	broken := application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithOAuth(repo), application.WithLoginLimiter(failingLimiter{}))
	if _, err := broken.RegisterClient(ctx, "198.51.100.9", "Bot", []string{"https://bot.example/cb"}); err == nil || errors.As(err, &tooMany) {
		t.Fatalf("limiter outage must fail closed: %v", err)
	}
}
