package server_test

import "testing"

// ADR-0003 phase 2: a project's own permission scheme, edited by its admins, decides for every context.
func TestProjectPermissionScheme(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		vic := anon.signedIn("vic@x.uz")
		eve := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		const url = "/api/v1/projects/KYB/permissions"

		code, scheme := lead.do("GET", url, nil)
		expect(t, code, 200, scheme)
		if scheme["custom"] != false || len(scheme["permissions"].([]any)) != 13 {
			t.Fatalf("default scheme = %v", scheme)
		}
		for who, want := range map[*client]int{dev: 403, eve: 404} {
			if code, b := who.do("GET", url, nil); code != want {
				t.Errorf("GET as non-admin: %d %v", code, b)
			}
			if code, b := who.do("DELETE", url, nil); code != want {
				t.Errorf("DELETE as non-admin: %d %v", code, b)
			}
		}

		// Viewers may create issues here; members may no longer start sprints.
		perms := scheme["permissions"].([]any)
		for _, p := range perms {
			p := p.(map[string]any)
			switch p["permission"] {
			case "CREATE_ISSUES":
				p["grants"] = append(p["grants"].([]any), map[string]any{"kind": "role", "value": "viewer"})
			case "MANAGE_SPRINTS":
				p["grants"] = []any{map[string]any{"kind": "role", "value": "admin"}}
			}
		}
		code, b := lead.do("PUT", url, map[string]any{"permissions": perms})
		expect(t, code, 200, b)
		if b["custom"] != true {
			t.Fatalf("saved = %v", b)
		}
		code, b = vic.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "from a viewer", "type": "task"})
		expect(t, code, 201, b)
		code, b = dev.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S1"})
		expect(t, code, 403, b)

		// Admins cannot be locked out; bad holders are rejected.
		for _, bad := range []map[string]any{
			{"permissions": []any{map[string]any{"permission": "BROWSE_PROJECTS", "grants": []any{map[string]any{"kind": "role", "value": "admin"}}}}},
			{"permissions": []any{map[string]any{"permission": "BROWSE_PROJECTS", "grants": []any{map[string]any{"kind": "role", "value": "owner"}}}}},
		} {
			code, b := lead.do("PUT", url, bad)
			expect(t, code, 422, b)
		}
		if code, b := lead.do("PUT", url, "not an object"); code != 400 {
			t.Fatalf("bad JSON: %d %v", code, b)
		}

		code, b = lead.do("DELETE", url, nil)
		expect(t, code, 204, b)
		code, b = vic.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "again", "type": "task"})
		expect(t, code, 403, b)
		_, scheme = lead.do("GET", url, nil)
		if scheme["custom"] != false {
			t.Fatalf("after reset = %v", scheme)
		}
	})
}
