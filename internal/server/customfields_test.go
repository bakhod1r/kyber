package server_test

import (
	"net/url"
	"testing"
)

// Custom fields: admins define them, members fill them in, JQL finds issues by them.
func TestCustomFields(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		code, b := dev.do("POST", "/api/v1/projects/KYB/fields", map[string]any{"name": "Tier", "type": "select", "options": []string{"Gold"}})
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/fields", map[string]any{"name": "Status", "type": "text"})
		expect(t, code, 422, b)
		code, tier := lead.do("POST", "/api/v1/projects/KYB/fields", map[string]any{"name": "Tier", "type": "select", "options": []string{"Gold", "Silver"}})
		expect(t, code, 201, tier)
		code, b = lead.do("POST", "/api/v1/projects/KYB/fields", map[string]any{"name": "tier", "type": "text"})
		expect(t, code, 409, b)
		_, due := lead.do("POST", "/api/v1/projects/KYB/fields", map[string]any{"name": "Deadline", "type": "date"})
		_, list := dev.do("GET", "/api/v1/projects/KYB/fields", nil)
		if len(list["items"].([]any)) != 2 {
			t.Fatalf("fields = %v", list)
		}
		tid, did := tier["id"].(string), due["id"].(string)

		_, is := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Deal", "type": "task"})
		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Other", "type": "task"})
		code, b = dev.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "custom_fields": map[string]string{tid: "Bronze"}})
		expect(t, code, 422, b)
		code, is = dev.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "custom_fields": map[string]string{tid: "gold", did: "2026-12-01"}})
		expect(t, code, 200, is)
		if cf := is["custom_fields"].(map[string]any); cf[tid] != "Gold" || cf[did] != "2026-12-01" {
			t.Fatalf("values = %v", cf)
		}
		for jql, want := range map[string]string{`Tier = gold`: "KYB-1", `Deadline < 2027-01-01`: "KYB-1", `tier is empty`: "KYB-2"} {
			code, b := dev.do("GET", "/api/v1/search?jql="+url.QueryEscape(jql), nil)
			expect(t, code, 200, b)
			if got := keysOf(b); got != want {
				t.Errorf("%q = %s, want %s", jql, got, want)
			}
		}
		code, b = lead.do("PUT", "/api/v1/fields/"+tid, map[string]any{"version": tier["version"], "name": "Level", "options": []string{"Gold", "Bronze"}})
		expect(t, code, 200, b)
		code, b = lead.do("DELETE", "/api/v1/fields/"+did, nil)
		expect(t, code, 204, b)
		code, b = dev.do("GET", "/api/v1/search?jql="+url.QueryEscape("Deadline < 2027-01-01"), nil)
		expect(t, code, 422, b) // the field is gone
		code, b = lead.do("DELETE", "/api/v1/fields/"+did, nil)
		expect(t, code, 404, b)
	})
}
