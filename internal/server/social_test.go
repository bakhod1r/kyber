package server_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/google"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/telegram"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/server"
)

// fakeGoogle stands in for the OIDC adapter (tested against a fake provider in its own package).
type fakeGoogle struct {
	profiles map[string]application.ExternalProfile
} // by code

func (fakeGoogle) Enabled() bool { return true }
func (fakeGoogle) AuthURL(_ context.Context, f google.Flow) (string, error) {
	return "https://accounts.google.test/auth?state=" + f.State, nil
}
func (g fakeGoogle) Exchange(_ context.Context, code string, _ google.Flow) (application.ExternalProfile, error) {
	p, ok := g.profiles[code]
	if !ok {
		return application.ExternalProfile{}, google.ErrInvalidLogin
	}
	return p, nil
}

const tgToken = "123:test-bot"

func tgPayload(id string) map[string]any {
	v := url.Values{"id": {id}, "first_name": {"Bobur"}, "username": {"bobur"}, "auth_date": {strconv.FormatInt(time.Now().Unix(), 10)}}
	keys := []string{}
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := []string{}
	for _, k := range keys {
		lines = append(lines, k+"="+v.Get(k))
	}
	secret := sha256.Sum256([]byte(tgToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	n, _ := strconv.Atoi(id)
	ad, _ := strconv.Atoi(v.Get("auth_date"))
	return map[string]any{"id": n, "first_name": "Bobur", "username": "bobur", "auth_date": ad, "hash": hex.EncodeToString(mac.Sum(nil))}
}

// browser sends a request without following redirects, carrying the given cookies.
func browser(t *testing.T, c *client, path string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", c.srv.URL+path, nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	spec.check(t, req, res)
	return res
}

func cookie(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestS39SocialSignIn(t *testing.T) {
	g := fakeGoogle{profiles: map[string]application.ExternalProfile{
		"new":      {Provider: domain.ProviderGoogle, Subject: "g-1", Email: "ali@gmail.com", EmailVerified: true, Name: "Ali G"},
		"takeover": {Provider: domain.ProviderGoogle, Subject: "g-2", Email: "dev@x.uz", EmailVerified: false, Name: "Evil"},
	}}
	social := server.WithSocial(identityhttp.Social{Google: g, Telegram: telegram.NewVerifier(tgToken, time.Now), TelegramBot: "kyber_bot"})
	forEachBackend(t, func(t *testing.T, anon *client) {
		code, p := anon.do("GET", "/api/v1/auth/providers", nil)
		expect(t, code, 200, p)
		if p["google"] != true || p["telegram_bot"] != "kyber_bot" {
			t.Fatalf("providers = %v", p)
		}

		// AC1 Google: start sets a flow cookie and redirects; callback signs in and lands on /.
		start := browser(t, anon, "/api/v1/auth/google/start")
		flow := cookie(start, "kyber_oauth")
		if start.StatusCode != 302 || flow == nil || !flow.HttpOnly || !strings.HasPrefix(start.Header.Get("Location"), "https://accounts.google.test/auth") {
			t.Fatalf("start = %d %v", start.StatusCode, start.Header)
		}
		state := strings.Split(flow.Value, ".")[0]
		cb := browser(t, anon, "/api/v1/auth/google/callback?code=new&state="+state, flow)
		session := cookie(cb, "kyber_session")
		if cb.StatusCode != 302 || cb.Header.Get("Location") != "/" || session == nil {
			t.Fatalf("callback = %d %v", cb.StatusCode, cb.Header)
		}
		me := &client{t: t, srv: anon.srv, token: session.Value}
		if code, u := me.do("GET", "/api/v1/me", nil); code != 200 || u["email"] != "ali@gmail.com" || u["name"] != "Ali G" {
			t.Fatalf("me = %d %v", code, u)
		}

		// AC2 login CSRF: a callback whose state does not match this browser's cookie is refused.
		for _, bad := range []*http.Response{
			browser(t, anon, "/api/v1/auth/google/callback?code=new&state=forged", flow),
			browser(t, anon, "/api/v1/auth/google/callback?code=new&state="+state), // no cookie
			browser(t, anon, "/api/v1/auth/google/callback?code=unknown&state="+state, flow),
		} {
			if bad.Header.Get("Location") != "/login?error=google_failed" || cookie(bad, "kyber_session") != nil {
				t.Fatalf("bad callback = %d %v", bad.StatusCode, bad.Header)
			}
		}

		// AC3 an unverified Google email cannot take over a password account.
		anon.signedIn("dev@x.uz")
		take := browser(t, anon, "/api/v1/auth/google/callback?code=takeover&state="+state, flow)
		if take.Header.Get("Location") != "/login?error=google_email_taken" {
			t.Fatalf("takeover = %v", take.Header)
		}

		// AC4 Telegram: a signed widget payload signs in; a tampered one is rejected.
		code, tok := anon.do("POST", "/api/v1/auth/telegram", tgPayload("42"))
		expect(t, code, 200, tok)
		tg := &client{t: t, srv: anon.srv, token: tok["token"].(string)}
		if code, u := tg.do("GET", "/api/v1/me", nil); code != 200 || u["name"] != "Bobur" {
			t.Fatalf("telegram me = %v", u)
		}
		forged := tgPayload("43")
		forged["id"] = 44
		code, b := anon.do("POST", "/api/v1/auth/telegram", forged)
		expect(t, code, 401, b)
	}, social)
}

// F6: a generic OpenID Connect provider (SSO) and GitHub sign in through the same redirect flow.
func TestF6SSOProviders(t *testing.T) {
	sso := fakeGoogle{profiles: map[string]application.ExternalProfile{
		"ok": {Provider: domain.ProviderOIDC, Subject: "o-1", Email: "dev@x.uz", EmailVerified: true, Name: "Dev"},
	}}
	gh := fakeGoogle{profiles: map[string]application.ExternalProfile{
		"ok":       {Provider: domain.ProviderGitHub, Subject: "583231", Email: "octo@x.uz", EmailVerified: true, Name: "Octo"},
		"takeover": {Provider: domain.ProviderGitHub, Subject: "9", Email: "dev@x.uz", EmailVerified: false, Name: "Evil"},
	}}
	social := server.WithSocial(identityhttp.Social{OIDC: sso, OIDCName: "Okta", GitHub: gh})
	forEachBackend(t, func(t *testing.T, anon *client) {
		code, p := anon.do("GET", "/api/v1/auth/providers", nil)
		expect(t, code, 200, p)
		if p["oidc"] != "Okta" || p["github"] != true || p["google"] != false {
			t.Fatalf("providers = %v", p)
		}
		_, dev := anon.signedIn("dev@x.uz").do("GET", "/api/v1/me", nil)
		for _, name := range []string{"oidc", "github"} {
			start := browser(t, anon, "/api/v1/auth/"+name+"/start")
			flow := cookie(start, "kyber_oauth")
			if start.StatusCode != 302 || flow == nil {
				t.Fatalf("%s start = %d %v", name, start.StatusCode, start.Header)
			}
			state := strings.Split(flow.Value, ".")[0]
			cb := browser(t, anon, "/api/v1/auth/"+name+"/callback?code=ok&state="+state, flow)
			session := cookie(cb, "kyber_session")
			if cb.Header.Get("Location") != "/" || session == nil {
				t.Fatalf("%s callback = %d %v", name, cb.StatusCode, cb.Header)
			}
			me := &client{t: t, srv: anon.srv, token: session.Value}
			_, u := me.do("GET", "/api/v1/me", nil)
			if name == "oidc" && u["id"] != dev["id"] { // a verified email links to the existing account
				t.Fatalf("sso me = %v, want %v", u, dev)
			}
			if bad := browser(t, anon, "/api/v1/auth/"+name+"/callback?code=ok&state=forged", flow); bad.Header.Get("Location") != "/login?error="+name+"_failed" {
				t.Fatalf("%s forged = %v", name, bad.Header)
			}
		}
		start := browser(t, anon, "/api/v1/auth/github/start")
		flow := cookie(start, "kyber_oauth")
		take := browser(t, anon, "/api/v1/auth/github/callback?code=takeover&state="+strings.Split(flow.Value, ".")[0], flow)
		if take.Header.Get("Location") != "/login?error=github_email_taken" {
			t.Fatalf("takeover = %v", take.Header)
		}
	}, social)
}

func TestS39SocialDisabled(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		code, p := anon.do("GET", "/api/v1/auth/providers", nil)
		expect(t, code, 200, p)
		if p["google"] != false || p["telegram_bot"] != nil || p["oidc"] != nil || p["github"] != false {
			t.Fatalf("providers = %v", p)
		}
		for _, path := range []string{"/api/v1/auth/google/start", "/api/v1/auth/google/callback", "/api/v1/auth/oidc/start", "/api/v1/auth/github/callback"} {
			if res := browser(t, anon, path); res.StatusCode != 404 {
				t.Fatalf("%s = %d", path, res.StatusCode)
			}
		}
		code, b := anon.do("POST", "/api/v1/auth/telegram", map[string]any{"id": 1})
		expect(t, code, 404, b)
	})
}
