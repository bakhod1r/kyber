package server_test

import "testing"

// S45: issue links — blocks, relates, duplicates, clones — read from both ends.
func TestS45Links(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "API", "type": "task"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "UI", "type": "task"})

		code, l := lead.do("POST", "/api/v1/issues/KYB-1/links", map[string]string{"type": "blocks", "to_key": "KYB-2"})
		expect(t, code, 201, l)
		code, b := lead.do("POST", "/api/v1/issues/KYB-1/links", map[string]string{"type": "blocks", "to_key": "KYB-2"})
		expect(t, code, 409, b)
		if b["code"] != "ISSUE_LINK_EXISTS" {
			t.Fatalf("problem = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/issues/KYB-1/links", map[string]string{"type": "causes", "to_key": "KYB-2"})
		expect(t, code, 422, b)

		code, list := lead.do("GET", "/api/v1/issues/KYB-2/links", nil)
		expect(t, code, 200, list)
		items := list["items"].([]any)
		first := items[0].(map[string]any)
		issue := first["issue"].(map[string]any)
		if len(items) != 1 || first["phrase"] != "is blocked by" || issue["key"] != "KYB-1" || issue["title"] != "API" {
			t.Fatalf("links of KYB-2 = %v", list)
		}
		code, b = anon.signedIn("alien@x.uz").do("DELETE", "/api/v1/links/"+first["id"].(string), nil)
		expect(t, code, 404, b)
		code, b = lead.do("DELETE", "/api/v1/links/"+first["id"].(string), nil)
		expect(t, code, 204, b)
		code, b = lead.do("DELETE", "/api/v1/links/"+first["id"].(string), nil)
		expect(t, code, 404, b)
		if b["code"] != "ISSUE_LINK_NOT_FOUND" {
			t.Fatalf("problem = %v", b)
		}
	})
}
