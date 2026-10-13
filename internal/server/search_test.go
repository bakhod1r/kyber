package server_test

import (
	"net/url"
	"strings"
	"testing"
)

func keysOf(b map[string]any) string {
	var out []string
	for _, x := range b["items"].([]any) {
		out = append(out, x.(map[string]any)["key"].(string))
	}
	return strings.Join(out, ",")
}

// JQL search across browsable projects, never leaking hidden issues; saved filters.
func TestSearchAndFilters(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "OPS", "name": "Ops"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		_, me := dev.do("GET", "/api/v1/me", nil)
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login crashes", "type": "bug"})
		_, two := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Sign in with Google", "type": "story"})
		lead.do("PATCH", "/api/v1/issues/KYB-2", map[string]any{"version": two["version"], "assignee_id": me["id"]})
		_, secret := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Salaries", "type": "task"})
		lead.do("PATCH", "/api/v1/issues/KYB-3", map[string]any{"version": secret["version"], "security_level": "confidential"})
		lead.do("POST", "/api/v1/projects/OPS/issues", map[string]string{"title": "Rotate keys", "type": "task"})
		lead.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})

		search := func(c *client, jql string) (int, map[string]any) {
			return c.do("GET", "/api/v1/search?jql="+url.QueryEscape(jql), nil)
		}
		for who, cases := range map[*client]map[string]string{
			lead: {"": "KYB-1,KYB-2,KYB-3,OPS-1", "project = OPS": "OPS-1", "type = bug": "KYB-1", "statusCategory = done": "KYB-1",
				"resolution is empty order by key desc": "OPS-1,KYB-3,KYB-2"},
			dev: {"": "KYB-1,KYB-2", "assignee = currentUser()": "KYB-2", `summary ~ "sign"`: "KYB-2", "text ~ salaries": "",
				`assignee = "dev@x.uz"`: "KYB-2", "project = OPS": ""},
		} {
			for jql, want := range cases {
				code, b := search(who, jql)
				expect(t, code, 200, b)
				if got := keysOf(b); got != want || b["total"] != float64(len(b["items"].([]any))) {
					t.Errorf("%q = %s (total %v), want %s", jql, got, b["total"], want)
				}
			}
		}
		code, b := search(dev, "project =")
		expect(t, code, 422, b)
		if !strings.Contains(b["detail"].(string), "JQL at 9") {
			t.Fatalf("detail = %v", b)
		}

		// Saved filters: private to the owner unless shared; only the owner changes them.
		code, f := dev.do("POST", "/api/v1/filters", map[string]any{"name": "My work", "jql": "assignee = currentUser()"})
		expect(t, code, 201, f)
		code, b = dev.do("POST", "/api/v1/filters", map[string]any{"name": "Bad", "jql": "colour = red"})
		expect(t, code, 422, b)
		_, list := lead.do("GET", "/api/v1/filters", nil)
		if len(list["items"].([]any)) != 0 {
			t.Fatalf("lead sees a private filter: %v", list)
		}
		code, f = dev.do("PUT", "/api/v1/filters/"+f["id"].(string), map[string]any{"version": f["version"], "name": "Team work", "jql": "project = KYB", "shared": true})
		expect(t, code, 200, f)
		_, list = lead.do("GET", "/api/v1/filters", nil)
		if items := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["mine"] != false {
			t.Fatalf("lead sees %v", list)
		}
		code, b = lead.do("DELETE", "/api/v1/filters/"+f["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = dev.do("PUT", "/api/v1/filters/"+f["id"].(string), map[string]any{"version": 1, "name": "x", "jql": ""})
		expect(t, code, 409, b)
		code, b = dev.do("DELETE", "/api/v1/filters/"+f["id"].(string), nil)
		expect(t, code, 204, b)
		code, b = dev.do("DELETE", "/api/v1/filters/"+f["id"].(string), nil)
		expect(t, code, 404, b)
	})
}
