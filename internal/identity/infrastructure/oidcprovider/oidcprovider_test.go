package oidcprovider_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/oidcprovider"
)

// fakeIdP is a minimal OpenID provider: discovery, JWKS and a token endpoint enforcing PKCE.
type fakeIdP struct {
	*httptest.Server
	key, rogue *rsa.PrivateKey
	challenge  string // from the authorization request
	claims     func(issuer string) map[string]any
	signWith   *rsa.PrivateKey
}

func newFake(t *testing.T) *fakeIdP {
	t.Helper()
	f := &fakeIdP{}
	f.key, _ = rsa.GenerateKey(rand.Reader, 2048)
	f.rogue, _ = rsa.GenerateKey(rand.Reader, 2048)
	f.signWith = f.key
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": f.URL, "authorization_endpoint": f.URL + "/auth",
			"token_endpoint": f.URL + "/token", "jwks_uri": f.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "good-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.signWith}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
		raw, _ := jwt.Signed(signer).Claims(f.claims(f.URL)).Serialize()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": raw})
	})
	return f
}

func validClaims(nonce string) func(string) map[string]any {
	return func(iss string) map[string]any {
		return map[string]any{"iss": iss, "aud": "kyber-client", "sub": "g-123", "nonce": nonce, "email": "ali@gmail.com",
			"email_verified": true, "name": "Ali Valiyev", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	}
}

func clientWith(f *fakeIdP, groupsClaim string) *oidcprovider.Client {
	return oidcprovider.New(oidcprovider.Config{Provider: domain.ProviderOIDC, Issuer: f.URL, ClientID: "kyber-client", ClientSecret: "s",
		RedirectURL: "https://kyber.test/cb", RequireVerifiedEmail: true, GroupsClaim: groupsClaim, AuthParams: map[string]string{"acr_values": "mfa"}})
}

func client(f *fakeIdP) *oidcprovider.Client {
	return oidcprovider.New(oidcprovider.Config{Provider: domain.ProviderOIDC, Issuer: f.URL, ClientID: "kyber-client", ClientSecret: "s",
		RedirectURL: "https://kyber.test/cb", RequireVerifiedEmail: true, AuthParams: map[string]string{"acr_values": "mfa"}})
}

// start runs the authorization request and records the PKCE challenge the fake will enforce.
func start(t *testing.T, f *fakeIdP, c *oidcprovider.Client, flow oidcprovider.Flow) {
	t.Helper()
	u, err := c.AuthURL(context.Background(), flow)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	v := q.Query()
	if !strings.HasPrefix(u, f.URL+"/auth?") || v.Get("state") != flow.State || v.Get("nonce") != flow.Nonce ||
		v.Get("code_challenge_method") != "S256" || !strings.Contains(v.Get("scope"), "email") || v.Get("redirect_uri") != "https://kyber.test/cb" || v.Get("acr_values") != "mfa" {
		t.Fatalf("auth url = %s", u)
	}
	f.challenge = v.Get("code_challenge")
}

func TestExchange(t *testing.T) {
	f := newFake(t)
	c := client(f)
	flow := application.NewOIDCFlow()
	if flow.State == flow.Nonce || len(flow.Verifier) < 43 {
		t.Fatalf("flow secrets = %+v", flow)
	}
	start(t, f, c, flow)
	f.claims = validClaims(flow.Nonce)
	p, err := c.Exchange(context.Background(), "good-code", flow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != domain.ProviderOIDC || p.Subject != "g-123" || p.Email != "ali@gmail.com" || !p.EmailVerified || p.Name != "Ali Valiyev" {
		t.Fatalf("profile = %+v", p)
	}
}

func TestExchangeRejects(t *testing.T) {
	cases := map[string]func(f *fakeIdP, flow *oidcprovider.Flow){
		"wrong nonce": func(f *fakeIdP, flow *oidcprovider.Flow) { f.claims = validClaims("other") },
		"other audience": func(f *fakeIdP, flow *oidcprovider.Flow) {
			f.claims = withClaim(validClaims(flow.Nonce), "aud", "someone-else")
		},
		"expired token": func(f *fakeIdP, flow *oidcprovider.Flow) {
			f.claims = withClaim(validClaims(flow.Nonce), "exp", time.Now().Add(-time.Hour).Unix())
		},
		"forged signature": func(f *fakeIdP, flow *oidcprovider.Flow) { f.claims = validClaims(flow.Nonce); f.signWith = f.rogue },
		"unverified email": func(f *fakeIdP, flow *oidcprovider.Flow) {
			f.claims = withClaim(validClaims(flow.Nonce), "email_verified", false)
		},
		"missing PKCE": func(f *fakeIdP, flow *oidcprovider.Flow) { f.claims = validClaims(flow.Nonce); flow.Verifier = "x" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			c := client(f)
			flow := application.NewOIDCFlow()
			start(t, f, c, flow)
			mutate(f, &flow)
			if _, err := c.Exchange(context.Background(), "good-code", flow); !errors.Is(err, oidcprovider.ErrInvalidLogin) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func withClaim(base func(string) map[string]any, k string, v any) func(string) map[string]any {
	return func(iss string) map[string]any {
		m := base(iss)
		m[k] = v
		return m
	}
}

func TestDiscoveryFailureAndConfig(t *testing.T) {
	down := oidcprovider.New(oidcprovider.Config{Provider: domain.ProviderOIDC, Issuer: "http://127.0.0.1:1", ClientID: "c", ClientSecret: "s"})
	if _, err := down.AuthURL(context.Background(), application.NewOIDCFlow()); err == nil {
		t.Fatal("unreachable issuer must fail")
	}
	if _, err := down.Exchange(context.Background(), "code", application.NewOIDCFlow()); err == nil {
		t.Fatal("unreachable issuer must fail")
	}
	if oidcprovider.New(oidcprovider.Config{ClientID: "c", ClientSecret: "s"}).Enabled() ||
		!oidcprovider.New(oidcprovider.Config{Issuer: "https://id.test", ClientID: "c", ClientSecret: "s"}).Enabled() {
		t.Fatal("enabled only with issuer, client id and secret")
	}
}

// Regression: the provider is discovered during /start, whose request context ends before
// /callback; Google's signing keys are fetched later and must not use that dead context.
func TestKeysFetchedAfterDiscoveryRequestEnded(t *testing.T) {
	f := newFake(t)
	c := client(f)
	flow := application.NewOIDCFlow()
	startCtx, cancel := context.WithCancel(context.Background())
	u, err := c.AuthURL(startCtx, flow)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // the /start request is over
	q, _ := url.Parse(u)
	f.challenge = q.Query().Get("code_challenge")
	f.claims = validClaims(flow.Nonce)
	if _, err := c.Exchange(context.Background(), "good-code", flow); err != nil {
		t.Fatalf("callback after start ended: %v", err)
	}
}

// S106: the groups claim (configurable; a list or a single string) becomes the profile's groups;
// without the claim the profile has none (nil: group sync is skipped).
func TestExchangeGroups(t *testing.T) {
	for name, c := range map[string]struct {
		claim string
		value any
		want  []string
	}{
		"default list":  {"", []any{"eng", "leads"}, []string{"eng", "leads"}},
		"custom claim":  {"roles", []any{"admins"}, []string{"admins"}},
		"single string": {"", "eng", []string{"eng"}},
		"absent":        {"", nil, nil},
		"empty list":    {"", []any{}, []string{}},
		"wrong type":    {"", 42, nil},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			cl := clientWith(f, c.claim)
			flow := application.NewOIDCFlow()
			start(t, f, cl, flow)
			claim := c.claim
			if claim == "" {
				claim = "groups"
			}
			f.claims = validClaims(flow.Nonce)
			if c.value != nil {
				f.claims = withClaim(validClaims(flow.Nonce), claim, c.value)
			}
			p, err := cl.Exchange(context.Background(), "good-code", flow)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(p.Groups) != fmt.Sprint(c.want) || (c.want == nil) != (p.Groups == nil) {
				t.Fatalf("groups = %#v, want %#v", p.Groups, c.want)
			}
		})
	}
}
