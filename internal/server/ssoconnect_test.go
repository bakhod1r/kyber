package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
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

func (p connIdP) Exchange(_ context.Context, code string, _ application.OIDCFlow) (application.ExternalProfile, error) {
	if code != "ok" {
		return application.ExternalProfile{}, application.ErrInvalidLogin
	}
	return application.ExternalProfile{Provider: p.provider, Subject: "okta-42", Email: "zara@acme.uz", EmailVerified: true, Name: "Zara"}, nil
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
