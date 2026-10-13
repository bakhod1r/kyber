package server_test

import (
	"strings"
	"testing"
)

// Forms: admins design intake forms; members submit them as new issues.
func TestForms(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})

		const url = "/api/v1/projects/KYB/forms"
		design := map[string]any{"name": "Bug report", "intro": "Tell us", "issue_type": "bug",
			"questions": []map[string]any{{"label": "Steps", "required": true, "long": true}, {"label": "Browser"}}}
		code, b := dev.do("POST", url, design)
		expect(t, code, 403, b)
		code, b = lead.do("POST", url, map[string]any{"name": "Epic maker", "issue_type": "epic"})
		expect(t, code, 422, b)
		code, f := lead.do("POST", url, design)
		expect(t, code, 201, f)
		furl := "/api/v1/forms/" + f["id"].(string)

		code, list := vic.do("GET", url, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 {
			t.Fatalf("forms = %v", list)
		}
		code, b = out.do("GET", furl, nil)
		expect(t, code, 404, b)
		code, f = vic.do("GET", furl, nil)
		expect(t, code, 200, f)

		code, b = dev.do("POST", furl+"/submissions", map[string]any{"summary": "Login fails", "answers": map[string]string{"q2": "Firefox"}})
		expect(t, code, 422, b)
		code, b = vic.do("POST", furl+"/submissions", map[string]any{"summary": "Login fails", "answers": map[string]string{"q1": "Click"}})
		expect(t, code, 403, b)
		code, sub := dev.do("POST", furl+"/submissions", map[string]any{"summary": "Login fails", "answers": map[string]string{"q1": "Click login", "q2": "Firefox"}})
		expect(t, code, 201, sub)
		code, is := dev.do("GET", "/api/v1/issues/"+sub["issue_key"].(string), nil)
		expect(t, code, 200, is)
		if is["title"] != "Login fails" || is["type"] != "bug" || !strings.Contains(is["description"].(string), "**Steps**\n\nClick login") {
			t.Fatalf("issue = %v", is)
		}

		code, b = lead.do("PUT", furl, map[string]any{"version": 9, "name": "x", "issue_type": "task"})
		expect(t, code, 409, b)
		code, f = lead.do("PUT", furl, map[string]any{"version": f["version"], "name": "Request", "issue_type": "task"})
		expect(t, code, 200, f)
		code, b = dev.do("DELETE", furl, nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", furl, nil)
		expect(t, code, 204, b)
	})
}
