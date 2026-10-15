package server_test

import (
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/server"
)

// S106: a site admin maps IdP groups to roles; every SSO login applies them and audits the change;
// leaving the group revokes what SSO granted, while roles given by hand stay.
func TestSSOGroupMapping(t *testing.T) {
	sso := fakeGoogle{profiles: map[string]application.ExternalProfile{
		"eng":  {Provider: domain.ProviderOIDC, Subject: "o-ann", Email: "ann@x.uz", EmailVerified: true, Name: "Ann", Groups: []string{"Engineers", "leads"}},
		"left": {Provider: domain.ProviderOIDC, Subject: "o-ann", Email: "ann@x.uz", EmailVerified: true, Name: "Ann", Groups: []string{}},
	}}
	forEachBackend(t, func(t *testing.T, anon *client) {
		boss := anon.signedIn("boss@x.uz")
		boss.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		boss.do("POST", "/api/v1/projects", map[string]string{"key": "OPS", "name": "Ops"})
		dev := anon.signedIn("dev@x.uz")
		code, b := dev.do("GET", "/api/v1/admin/sso/group-mappings", nil)
		expect(t, code, 403, b)

		code, m := boss.do("POST", "/api/v1/admin/sso/group-mappings", map[string]string{"group": "engineers", "scope": "project", "project_key": "KYB", "role": "member"})
		expect(t, code, 201, m)
		boss.do("POST", "/api/v1/admin/sso/group-mappings", map[string]string{"group": "leads", "scope": "project", "project_key": "KYB", "role": "admin"})
		code, b = boss.do("POST", "/api/v1/admin/sso/group-mappings", map[string]string{"group": "x", "scope": "project", "project_key": "KYB", "role": "boss"})
		expect(t, code, 422, b)
		code, list := boss.do("GET", "/api/v1/admin/sso/group-mappings", nil)
		expect(t, code, 200, list)
		if len(list["items"].([]any)) != 2 {
			t.Fatalf("mappings = %v", list)
		}

		login := func(profile string) *client {
			start := browser(t, anon, "/api/v1/auth/oidc/start")
			flow := cookie(start, "kyber_oauth")
			cb := browser(t, anon, "/api/v1/auth/oidc/callback?code="+profile+"&state="+strings.Split(flow.Value, ".")[0], flow)
			return &client{t: t, srv: anon.srv, token: cookie(cb, "kyber_session").Value}
		}
		ann := login("eng")
		_, me := ann.do("GET", "/api/v1/me", nil)
		roleOf := func(key string) string {
			_, ms := boss.do("GET", "/api/v1/projects/"+key+"/members", nil)
			for _, x := range ms["items"].([]any) {
				if x.(map[string]any)["user_id"] == me["id"] {
					return x.(map[string]any)["role"].(string)
				}
			}
			return ""
		}
		if r := roleOf("KYB"); r != "admin" {
			t.Fatalf("highest mapped role = %q", r)
		}
		// A role given by hand is not SSO's to take away.
		boss.do("POST", "/api/v1/projects/OPS/members", map[string]string{"email": "ann@x.uz", "role": "viewer"})
		_, audit := boss.do("GET", "/api/v1/projects/KYB/audit", nil)
		if !strings.Contains(fmt100(audit), "sso.role_granted") {
			t.Fatalf("audit = %v", audit)
		}

		login("left")
		if r := roleOf("KYB"); r != "" {
			t.Fatalf("left the groups but still %q on KYB", r)
		}
		if r := roleOf("OPS"); r != "viewer" {
			t.Fatalf("manual role touched: %q", r)
		}
		code, b = boss.do("DELETE", "/api/v1/admin/sso/group-mappings/"+m["id"].(string), nil)
		if code != 204 {
			t.Fatalf("delete = %d %v", code, b)
		}
		code, b = boss.do("DELETE", "/api/v1/admin/sso/group-mappings/"+m["id"].(string), nil)
		expect(t, code, 404, b)
	}, server.WithSiteAdmins("boss@x.uz"), server.WithSocial(identityhttp.Social{OIDC: sso, OIDCName: "Okta"}))
}

func fmt100(v any) string {
	var b strings.Builder
	var walk func(any)
	walk = func(x any) {
		switch y := x.(type) {
		case map[string]any:
			for _, z := range y {
				walk(z)
			}
		case []any:
			for _, z := range y {
				walk(z)
			}
		case string:
			b.WriteString(y + " ")
		}
	}
	walk(v)
	return b.String()
}
