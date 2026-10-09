package google_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/bakhod1r/kyber/internal/identity/adapter/google"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// fakeGoogle is a minimal OpenID provider: discovery, JWKS and a token endpoint enforcing PKCE.
type fakeGoogle struct {
	*httptest.Server
	key, rogue *rsa.PrivateKey
	challenge  string // from the authorization request
	claims     func(issuer string) map[string]any
	signWith   *rsa.PrivateKey
}

func newFake(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{}
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

func client(f *fakeGoogle) *google.Client {
	return google.New(google.Config{Issuer: f.URL, ClientID: "kyber-client", ClientSecret: "s", RedirectURL: "https://kyber.test/cb"})
}

// start runs the authorization request and records the PKCE challenge the fake will enforce.
func start(t *testing.T, f *fakeGoogle, c *google.Client, flow google.Flow) {
	t.Helper()
	u, err := c.AuthURL(context.Background(), flow)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	v := q.Query()
	if !strings.HasPrefix(u, f.URL+"/auth?") || v.Get("state") != flow.State || v.Get("nonce") != flow.Nonce ||
		v.Get("code_challenge_method") != "S256" || !strings.Contains(v.Get("scope"), "email") || v.Get("redirect_uri") != "https://kyber.test/cb" {
		t.Fatalf("auth url = %s", u)
	}
	f.challenge = v.Get("code_challenge")
}

func TestExchange(t *testing.T) {
	f := newFake(t)
	c := client(f)
	flow := google.NewFlow()
	if flow.State == flow.Nonce || len(flow.Verifier) < 43 {
		t.Fatalf("flow secrets = %+v", flow)
	}
	start(t, f, c, flow)
	f.claims = validClaims(flow.Nonce)
	p, err := c.Exchange(context.Background(), "good-code", flow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != domain.ProviderGoogle || p.Subject != "g-123" || p.Email != "ali@gmail.com" || !p.EmailVerified || p.Name != "Ali Valiyev" {
		t.Fatalf("profile = %+v", p)
	}
}

func TestExchangeRejects(t *testing.T) {
	cases := map[string]func(f *fakeGoogle, flow *google.Flow){
		"wrong nonce": func(f *fakeGoogle, flow *google.Flow) { f.claims = validClaims("other") },
		"other audience": func(f *fakeGoogle, flow *google.Flow) {
			f.claims = withClaim(validClaims(flow.Nonce), "aud", "someone-else")
		},
		"expired token": func(f *fakeGoogle, flow *google.Flow) {
			f.claims = withClaim(validClaims(flow.Nonce), "exp", time.Now().Add(-time.Hour).Unix())
		},
		"forged signature": func(f *fakeGoogle, flow *google.Flow) { f.claims = validClaims(flow.Nonce); f.signWith = f.rogue },
		"missing PKCE":     func(f *fakeGoogle, flow *google.Flow) { f.claims = validClaims(flow.Nonce); flow.Verifier = "x" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			c := client(f)
			flow := google.NewFlow()
			start(t, f, c, flow)
			mutate(f, &flow)
			if _, err := c.Exchange(context.Background(), "good-code", flow); !errors.Is(err, google.ErrInvalidLogin) {
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
	down := google.New(google.Config{Issuer: "http://127.0.0.1:1", ClientID: "c", ClientSecret: "s"})
	if _, err := down.AuthURL(context.Background(), google.NewFlow()); err == nil {
		t.Fatal("unreachable issuer must fail")
	}
	if _, err := down.Exchange(context.Background(), "code", google.NewFlow()); err == nil {
		t.Fatal("unreachable issuer must fail")
	}
	if google.New(google.Config{}).Enabled() || !google.New(google.Config{ClientID: "c", ClientSecret: "s"}).Enabled() {
		t.Fatal("enabled only with client id and secret")
	}
}

// Regression: the provider is discovered during /start, whose request context ends before
// /callback; Google's signing keys are fetched later and must not use that dead context.
func TestKeysFetchedAfterDiscoveryRequestEnded(t *testing.T) {
	f := newFake(t)
	c := client(f)
	flow := google.NewFlow()
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
