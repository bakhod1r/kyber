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

// S86: comments on a page's paragraphs; readers comment, @mentions of project members notify, authors remove.
func TestDocComments(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		_, p := lead.do("POST", "/api/v1/projects/KYB/docs", map[string]string{"title": "Guide", "body": "# Hi\n\nCache it"})
		url := "/api/v1/docs/" + p["id"].(string) + "/comments"

		code, c := lead.do("POST", url, map[string]any{"paragraph": 1, "quote": "Cache it", "body": "@vic@x.uz @out@x.uz why?"})
		expect(t, code, 201, c)
		if c["paragraph"] != float64(1) || c["quote"] != "Cache it" || c["author_id"] == "" {
			t.Fatalf("comment = %v", c)
		}
		code, x := lead.do("POST", url, map[string]any{"paragraph": -1, "body": "x"})
		expect(t, code, 422, x)
		code, x = out.do("POST", url, map[string]any{"paragraph": 0, "body": "x"})
		expect(t, code, 404, x)
		code, x = out.do("GET", url, nil)
		expect(t, code, 404, x)
		code, l := vic.do("GET", url, nil)
		expect(t, code, 200, l)
		if len(l["items"].([]any)) != 1 {
			t.Fatalf("comments = %v", l)
		}
		eventuallyJSON(t, vic, "/api/v1/notifications", func(n map[string]any) bool {
			items := n["items"].([]any)
			return len(items) == 1 && items[0].(map[string]any)["kind"] == "doc"
		})
		if _, n := out.do("GET", "/api/v1/notifications", nil); len(n["items"].([]any)) != 0 {
			t.Fatalf("an outsider was told: %v", n)
		}
		del := "/api/v1/doc-comments/" + c["id"].(string)
		code, x = vic.do("DELETE", del, nil)
		expect(t, code, 403, x)
		code, x = out.do("DELETE", del, nil)
		expect(t, code, 404, x)
		code, x = lead.do("DELETE", del, nil)
		expect(t, code, 204, x)
		code, x = lead.do("DELETE", del, nil)
		expect(t, code, 404, x)
	})
}
