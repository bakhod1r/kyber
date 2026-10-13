package server_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

// Admin panel in single-tenant mode: configured administrators see members, projects and
// stuck events; roles are configuration, not editable.
func TestAdminPanelSingleTenant(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		boss := anon.signedIn("boss@x.uz")
		dev := anon.signedIn("dev@x.uz")
		dev.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		boss.do("GET", "/api/v1/me", nil)

		code, a := boss.do("GET", "/api/v1/admin/access", nil)
		expect(t, code, 200, a)
		if a["admin"] != true {
			t.Fatalf("boss access = %v", a)
		}
		code, a = dev.do("GET", "/api/v1/admin/access", nil)
		expect(t, code, 200, a)
		if a["admin"] != false {
			t.Fatalf("dev access = %v", a)
		}
		for _, path := range []string{"/api/v1/admin/members", "/api/v1/admin/projects", "/api/v1/admin/dead-letters"} {
			code, b := dev.do("GET", path, nil)
			expect(t, code, 403, b)
		}

		code, ms := boss.do("GET", "/api/v1/admin/members", nil)
		expect(t, code, 200, ms)
		if items := ms["items"].([]any); len(items) != 1 || items[0].(map[string]any)["email"] != "dev@x.uz" {
			t.Fatalf("members = %v", ms)
		}
		devID := ms["items"].([]any)[0].(map[string]any)["id"].(string)
		code, b := boss.do("PUT", "/api/v1/admin/members/"+devID, map[string]string{"role": "admin"})
		expect(t, code, 409, b)

		code, ps := boss.do("GET", "/api/v1/admin/projects", nil)
		expect(t, code, 200, ps)
		if items := ps["items"].([]any); len(items) != 1 || items[0].(map[string]any)["members"] != float64(1) {
			t.Fatalf("projects = %v", ps)
		}
		code, dl := boss.do("GET", "/api/v1/admin/dead-letters", nil)
		expect(t, code, 200, dl)
		if items := dl["items"].([]any); len(items) != 0 {
			t.Fatalf("dead letters = %v", dl)
		}
		for _, id := range []string{"999", "x"} {
			code, b = boss.do("POST", "/api/v1/admin/dead-letters/"+id+"/retry", nil)
			expect(t, code, 404, b)
		}
	}, server.WithSiteAdmins("boss@x.uz"))
}

// Admin panel on a named workspace: its owners and admins change roles.
func TestAdminPanelWorkspaceRoles(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		apex := on(anon, "kyber.test")
		alice := apex.signedIn("alice@x.uz")
		bob := apex.signedIn("bob@x.uz")
		alice.do("POST", "/api/v1/workspaces", map[string]string{"slug": "acme", "name": "Acme"})
		acme := on(alice, "acme.kyber.test")
		acme.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		acme.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "member"})
		bobAcme := on(bob, "acme.kyber.test")

		code, b := bobAcme.do("GET", "/api/v1/admin/members", nil)
		expect(t, code, 403, b)
		code, ms := acme.do("GET", "/api/v1/admin/members", nil)
		expect(t, code, 200, ms)
		var aliceID, bobID string
		for _, it := range ms["items"].([]any) {
			m := it.(map[string]any)
			switch m["email"] {
			case "alice@x.uz":
				aliceID = m["id"].(string)
			case "bob@x.uz":
				bobID = m["id"].(string)
			}
		}
		code, b = acme.do("PUT", "/api/v1/admin/members/"+aliceID, map[string]string{"role": "member"})
		expect(t, code, 409, b) // the last owner stays
		code, b = acme.do("PUT", "/api/v1/admin/members/"+bobID, map[string]string{"role": "boss"})
		expect(t, code, 422, b)
		code, b = acme.do("PUT", "/api/v1/admin/members/00000000-0000-4000-8000-000000000999", map[string]string{"role": "admin"})
		expect(t, code, 404, b)
		code, b = acme.do("PUT", "/api/v1/admin/members/"+bobID, map[string]string{"role": "admin"})
		expect(t, code, 204, b)
		code, a := bobAcme.do("GET", "/api/v1/admin/access", nil)
		expect(t, code, 200, a)
		if a["admin"] != true {
			t.Fatalf("bob is an admin now: %v", a)
		}
	}, server.WithBaseDomain("kyber.test"))
}
