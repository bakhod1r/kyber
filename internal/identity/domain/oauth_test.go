package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func TestNewOAuthClient(t *testing.T) {
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	c, err := domain.NewOAuthClient("c-1", " Claude ", []string{"https://claude.ai/api/mcp/auth_callback", "http://localhost:33418/callback", "com.example.app:/cb"}, at)
	if err != nil || c.Name != "Claude" || len(c.RedirectURIs) != 3 {
		t.Fatalf("client = %+v %v", c, err)
	}
	for _, uri := range []string{"https://claude.ai/api/mcp/auth_callback", "http://localhost:51000/callback", "com.example.app:/cb"} {
		if !c.AllowsRedirect(uri) {
			t.Errorf("AllowsRedirect(%q) = false", uri)
		}
	}
	for _, uri := range []string{"https://claude.ai/api/mcp/other", "http://localhost:1/elsewhere", "https://evil.example/cb", "http://127.0.0.1:9/callback", ""} {
		if c.AllowsRedirect(uri) {
			t.Errorf("AllowsRedirect(%q) = true", uri)
		}
	}
	bad := [][]string{nil, {"http://example.com/cb"}, {"https://x.example/cb#frag"}, {"javascript:alert(1)"}, {"/relative"}, {"myapp:/cb"}}
	for _, uris := range bad {
		if _, err := domain.NewOAuthClient("c", "App", uris, at); !errors.Is(err, domain.ErrInvalidOAuthClient) {
			t.Errorf("redirects %v: %v", uris, err)
		}
	}
	if _, err := domain.NewOAuthClient("c", "", []string{"https://x.example/cb"}, at); !errors.Is(err, domain.ErrInvalidOAuthClient) {
		t.Errorf("blank name: %v", err)
	}
}

func TestAuthCodePKCE(t *testing.T) {
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	code, err := domain.NewAuthCode([]byte("h"), "c-1", "u-1", "https://x.example/cb", challenge, at)
	if err != nil {
		t.Fatal(err)
	}
	if !code.Verifier(verifier) || code.Verifier(strings.Repeat("w", 43)) || code.Verifier("short") {
		t.Fatal("PKCE verification")
	}
	if code.Expired(at.Add(9*time.Minute)) || !code.Expired(at.Add(10*time.Minute)) {
		t.Fatal("codes live ten minutes")
	}
	if _, err := domain.NewAuthCode([]byte("h"), "c-1", "u-1", "https://x.example/cb", "not-a-challenge", at); !errors.Is(err, domain.ErrInvalidOAuthRequest) {
		t.Fatalf("bad challenge: %v", err)
	}
}
