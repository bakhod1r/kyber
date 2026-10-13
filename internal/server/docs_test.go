package server_test

import "testing"

// Docs: a project's Markdown pages with revision history.
func TestDocs(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})

		const url = "/api/v1/projects/KYB/docs"
		code, b := lead.do("POST", url, map[string]string{"title": " "})
		expect(t, code, 422, b)
		code, b = vic.do("POST", url, map[string]string{"title": "Mine"})
		expect(t, code, 403, b)
		code, p := lead.do("POST", url, map[string]string{"title": "Guide", "body": "# Hello"})
		expect(t, code, 201, p)
		purl := "/api/v1/docs/" + p["id"].(string)

		code, list := vic.do("GET", url, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 {
			t.Fatalf("pages = %v", list)
		}
		code, b = out.do("GET", purl, nil)
		expect(t, code, 404, b)
		code, b = vic.do("PUT", purl, map[string]any{"version": p["version"], "title": "x"})
		expect(t, code, 403, b)
		code, b = lead.do("PUT", purl, map[string]any{"version": 9, "title": "x"})
		expect(t, code, 409, b)
		code, p = lead.do("PUT", purl, map[string]any{"version": p["version"], "title": "Guide", "body": "## Two"})
		expect(t, code, 200, p)
		if p["version"] != float64(2) {
			t.Fatalf("page = %v", p)
		}
		code, p = vic.do("GET", purl, nil)
		expect(t, code, 200, p)
		code, h := vic.do("GET", purl+"/history", nil)
		expect(t, code, 200, h)
		if items := h["items"].([]any); len(items) != 2 || items[0].(map[string]any)["body"] != "## Two" {
			t.Fatalf("history = %v", h)
		}
		code, b = out.do("GET", purl+"/history", nil)
		expect(t, code, 404, b)
		code, b = lead.do("DELETE", purl, nil)
		expect(t, code, 204, b)
		code, b = lead.do("GET", purl, nil)
		expect(t, code, 404, b)
	})
}
