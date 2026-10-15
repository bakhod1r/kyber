package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/seal"
	"github.com/bakhod1r/kyber/internal/server"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// connIdP is a fake IdP per stored connection: identities carry that connection's provider name.
type connIdP struct {
	provider identitydomain.Provider
	issuer   string
}

func (p connIdP) AuthURL(_ context.Context, f application.OIDCFlow) (string, error) {
	return p.issuer + "/authorize?state=" + f.State, nil
}

// Exchange accepts "ok" (Zara) or an email address as the code: that person signs in.
func (p connIdP) Exchange(_ context.Context, code string, _ application.OIDCFlow) (application.ExternalProfile, error) {
	email := map[bool]string{true: "zara@acme.uz", false: code}[code == "ok"]
	if !strings.Contains(email, "@") {
		return application.ExternalProfile{}, application.ErrInvalidLogin
	}
	return application.ExternalProfile{Provider: p.provider, Subject: "okta-" + email, Email: email, EmailVerified: true, Name: "SSO user"}, nil
}

// S107 (ADR-0018 a): a site admin stores an identity provider; its button appears on the login
// page and signs people in through its own routes; the secret never comes back; disabling it
// removes the button.
func TestSSOConnections(t *testing.T) {
	box, err := seal.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	build := func(c domain.Connection) application.RedirectProvider {
		return connIdP{provider: identitydomain.Provider(domain.ProviderOf(c.ID)), issuer: c.Issuer}
	}
	forEachBackend(t, func(t *testing.T, anon *client) {
		boss := anon.signedIn("boss@x.uz")
		dev := anon.signedIn("dev@x.uz")
		okta := map[string]string{"name": "Acme Okta", "issuer": "https://acme.okta.com", "client_id": "kyber", "client_secret": "top-secret"}
		code, b := dev.do("POST", "/api/v1/admin/sso/connections", okta)
		expect(t, code, 403, b)
		code, c := boss.do("POST", "/api/v1/admin/sso/connections", okta)
		expect(t, code, 201, c)
		if _, leaked := c["client_secret"]; leaked || c["enabled"] != true || c["kind"] != "oidc" {
			t.Fatalf("created = %v", c)
		}
		id := c["id"].(string)
		bad := map[string]string{"name": "X", "issuer": "http://idp.example.com", "client_id": "a", "client_secret": "b"}
		code, b = boss.do("POST", "/api/v1/admin/sso/connections", bad)
		expect(t, code, 422, b)
		code, list := boss.do("GET", "/api/v1/admin/sso/connections", nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 || strings.Contains(fmt100(list), "top-secret") {
			t.Fatalf("list = %v", list)
		}

		_, prov := anon.do("GET", "/api/v1/auth/providers", nil)
		if sso := prov["sso"].([]any); len(sso) != 1 || sso[0].(map[string]any)["id"] != id || sso[0].(map[string]any)["name"] != "Acme Okta" {
			t.Fatalf("providers = %v", prov)
		}
		start := browser(t, anon, "/api/v1/auth/sso/"+id+"/start")
		flow := cookie(start, "kyber_oauth")
		if start.StatusCode != 302 || !strings.HasPrefix(start.Header.Get("Location"), "https://acme.okta.com/authorize") || flow.Path != "/api/v1/auth/sso/"+id+"/" {
			t.Fatalf("start = %d %s %+v", start.StatusCode, start.Header.Get("Location"), flow)
		}
		state := strings.Split(flow.Value, ".")[0]
		forged := browser(t, anon, "/api/v1/auth/sso/"+id+"/callback?code=ok&state=forged", flow)
		if loc := forged.Header.Get("Location"); loc != "/login?error=sso_failed" {
			t.Fatalf("forged state = %s", loc)
		}
		cb := browser(t, anon, "/api/v1/auth/sso/"+id+"/callback?code=ok&state="+state, flow)
		zara := &client{t: t, srv: anon.srv, token: cookie(cb, "kyber_session").Value}
		if code, me := zara.do("GET", "/api/v1/me", nil); code != 200 || me["email"] != "zara@acme.uz" {
			t.Fatalf("signed in = %d %v", code, me)
		}

		code, c = boss.do("PATCH", "/api/v1/admin/sso/connections/"+id, map[string]any{"enabled": false, "name": "Okta (old)"})
		expect(t, code, 200, c)
		if c["enabled"] != false || c["name"] != "Okta (old)" {
			t.Fatalf("updated = %v", c)
		}
		_, prov = anon.do("GET", "/api/v1/auth/providers", nil)
		if len(prov["sso"].([]any)) != 0 {
			t.Fatalf("disabled connection still offered: %v", prov)
		}
		code, b = anon.do("GET", "/api/v1/auth/sso/"+id+"/start", nil)
		expect(t, code, 404, b)
		code, b = boss.do("DELETE", "/api/v1/admin/sso/connections/"+id, nil)
		if code != 204 {
			t.Fatalf("delete = %d %v", code, b)
		}
		code, b = boss.do("DELETE", "/api/v1/admin/sso/connections/"+id, nil)
		expect(t, code, 404, b)
	}, server.WithSiteAdmins("boss@x.uz"), server.WithEncryptionKey(box), server.WithSSOProviders(build))
}

// fixedDNS serves TXT records from a map.
type fixedDNS map[string][]string

func (d fixedDNS) TXT(_ context.Context, name string) ([]string, error) { return d[name], nil }

// S107 (ADR-0018 b): a connection lists email domains; a domain routes sign-in only once its TXT
// record proves control (or a site admin trusts it); the login page discovers the IdP by email.
func TestSSODomainDiscovery(t *testing.T) {
	box, _ := seal.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	records := fixedDNS{}
	forEachBackend(t, func(t *testing.T, anon *client) {
		boss := anon.signedIn("boss@x.uz")
		discover := func(email string) map[string]any {
			t.Helper()
			code, b := anon.do("POST", "/api/v1/auth/sso/discover", map[string]string{"email": email})
			expect(t, code, 200, b)
			c, _ := b["connection"].(map[string]any)
			return c
		}
		_, c := boss.do("POST", "/api/v1/admin/sso/connections", map[string]string{"name": "Acme Okta", "issuer": "https://acme.okta.com", "client_id": "k", "client_secret": "s"})
		id := c["id"].(string)
		base := "/api/v1/admin/sso/connections/" + id + "/domains"
		code, c := boss.do("POST", base, map[string]any{"domain": "Acme.UZ"})
		expect(t, code, 200, c)
		d := c["domains"].([]any)[0].(map[string]any)
		txt := d["txt"].(string)
		if d["name"] != "acme.uz" || d["verified"] != false || !strings.HasPrefix(txt, "kyber-verify=") || len(txt) < 30 {
			t.Fatalf("domain = %v", d)
		}
		if found := discover("zara@acme.uz"); found != nil {
			t.Fatalf("an unverified domain routes: %v", found)
		}
		code, b := boss.do("POST", base+"/acme.uz/verify", nil)
		expect(t, code, 409, b)
		records["acme.uz"] = []string{"v=spf1 -all", txt}
		code, c = boss.do("POST", base+"/acme.uz/verify", nil)
		expect(t, code, 200, c)
		if found := discover("Zara@ACME.uz"); found == nil || found["id"] != id || found["name"] != "Acme Okta" {
			t.Fatalf("discover = %v", found)
		}
		if found := discover("zara@other.uz"); found != nil {
			t.Fatalf("other domain = %v", found)
		}
		_, other := boss.do("POST", "/api/v1/admin/sso/connections", map[string]string{"name": "Entra", "issuer": "https://login.example.com", "client_id": "k", "client_secret": "s"})
		code, b = boss.do("POST", "/api/v1/admin/sso/connections/"+other["id"].(string)+"/domains", map[string]any{"domain": "acme.uz"})
		expect(t, code, 409, b)
		code, c = boss.do("POST", "/api/v1/admin/sso/connections/"+other["id"].(string)+"/domains", map[string]any{"domain": "corp.uz", "trusted": true})
		expect(t, code, 200, c)
		if found := discover("ali@corp.uz"); found == nil || found["name"] != "Entra" {
			t.Fatalf("trusted domain = %v", found)
		}
		dev := anon.signedIn("dev@x.uz")
		code, b = dev.do("POST", base, map[string]any{"domain": "evil.uz", "trusted": true})
		expect(t, code, 403, b)
		code, c = boss.do("DELETE", base+"/acme.uz", nil)
		expect(t, code, 200, c)
		if found := discover("zara@acme.uz"); found != nil {
			t.Fatalf("removed domain routes: %v", found)
		}
		code, b = boss.do("DELETE", base+"/acme.uz", nil)
		expect(t, code, 404, b)
		req, _ := http.NewRequest("POST", anon.srv.URL+"/api/v1/auth/sso/discover", strings.NewReader("email=zara@acme.uz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("form post = %d", res.StatusCode)
		}
	}, server.WithSiteAdmins("boss@x.uz"), server.WithEncryptionKey(box), server.WithDNS(records))
}

// ssoLogin signs in through a stored connection on host and returns the session token.
func ssoLogin(t *testing.T, c *client, host, id, code string) *client {
	t.Helper()
	get := func(path string, ck *http.Cookie) *http.Response {
		req, _ := http.NewRequest("GET", c.srv.URL+path, nil)
		req.Host = host
		if ck != nil {
			req.AddCookie(ck)
		}
		res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	flow := cookie(get("/api/v1/auth/sso/"+id+"/start", nil), "kyber_oauth")
	cb := get("/api/v1/auth/sso/"+id+"/callback?code="+url.QueryEscape(code)+"&state="+strings.Split(flow.Value, ".")[0], flow)
	return &client{t: t, srv: c.srv, token: cookie(cb, "kyber_session").Value, host: c.host}
}

// S107 (ADR-0018 c): a workspace admin adds the workspace's own connection and requires it. Members
// signed in another way (or with an API token) are refused with SSO_REQUIRED and told where to sign
// in; signed in through the connection they get in; owners always keep a way in.
func TestSSOEnforcement(t *testing.T) {
	box, _ := seal.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	build := func(c domain.Connection) application.RedirectProvider {
		return connIdP{provider: identitydomain.Provider(domain.ProviderOf(c.ID)), issuer: c.Issuer}
	}
	forEachBackend(t, func(t *testing.T, anon *client) {
		apex := on(anon, "kyber.test")
		owner := apex.signedIn("owner@acme.uz")
		dev := apex.signedIn("dev@acme.uz")
		owner.do("POST", "/api/v1/workspaces", map[string]string{"slug": "acme", "name": "Acme"})
		acme := on(owner, "acme.kyber.test")
		acme.do("POST", "/api/v1/projects", map[string]string{"key": "ACM", "name": "Acme"})
		code, b := acme.do("POST", "/api/v1/projects/ACM/members", map[string]string{"email": "dev@acme.uz", "role": "member"})
		expect(t, code, 204, b)
		devAcme := on(dev, "acme.kyber.test")
		if code, b := devAcme.do("GET", "/api/v1/projects", nil); code != 200 {
			t.Fatalf("before = %d %v", code, b)
		}
		_, tok := devAcme.do("POST", "/api/v1/me/tokens", map[string]any{"name": "cli", "expires_in_days": 30})

		code, b = devAcme.do("POST", "/api/v1/admin/sso/connections", map[string]string{"name": "Acme Okta", "issuer": "https://acme.okta.com", "client_id": "k", "client_secret": "s", "scope": "workspace"})
		expect(t, code, 403, b)
		code, c := acme.do("POST", "/api/v1/admin/sso/connections", map[string]string{"name": "Acme Okta", "issuer": "https://acme.okta.com", "client_id": "k", "client_secret": "s", "scope": "workspace"})
		expect(t, code, 201, c)
		id := c["id"].(string)
		if c["workspace_id"] == nil || c["enforced"] != false {
			t.Fatalf("created = %v", c)
		}
		code, c = acme.do("PATCH", "/api/v1/admin/sso/connections/"+id, map[string]any{"enforced": true})
		expect(t, code, 200, c)

		code, b = devAcme.do("GET", "/api/v1/projects", nil)
		expect(t, code, 403, b)
		if b["code"] != "SSO_REQUIRED" || !strings.Contains(fmt100(b), "/api/v1/auth/sso/"+id+"/start") {
			t.Fatalf("refusal = %v", b)
		}
		pat := &client{t: t, srv: anon.srv, token: tok["token"].(string), host: "acme.kyber.test"}
		if code, b := pat.do("GET", "/api/v1/projects", nil); code != 403 {
			t.Fatalf("token = %d %v", code, b)
		}
		if code, _ := on(dev, "kyber.test").do("GET", "/api/v1/me", nil); code != 200 {
			t.Fatalf("the apex stays open: %d", code)
		}
		if code, b := acme.do("GET", "/api/v1/projects", nil); code != 200 {
			t.Fatalf("owner = %d %v", code, b)
		}

		viaSSO := ssoLogin(t, on(anon, "acme.kyber.test"), "kyber.test", id, "dev@acme.uz")
		if code, b := viaSSO.do("GET", "/api/v1/projects", nil); code != 200 {
			t.Fatalf("signed in through SSO = %d %v", code, b)
		}
		code, c = acme.do("PATCH", "/api/v1/admin/sso/connections/"+id, map[string]any{"enforced": false})
		expect(t, code, 200, c)
		if code, b := devAcme.do("GET", "/api/v1/projects", nil); code != 200 {
			t.Fatalf("after lifting = %d %v", code, b)
		}
	}, server.WithBaseDomain("kyber.test"), server.WithEncryptionKey(box), server.WithSSOProviders(build))
}
