package server_test

import "testing"

// B10: an archived project stays readable but nobody can change it (issues, sprints, members) until an
// admin restores it.
func TestArchiveProject(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		lead := c.signedIn("lead@x.uz")
		dev := c.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		code, b := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		expect(t, code, 201, b)

		code, b = dev.do("POST", "/api/v1/projects/KYB/archive", nil)
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/archive", nil)
		expect(t, code, 200, b)
		if b["archived"] != true {
			t.Fatalf("archive = %v", b)
		}
		code, b = dev.do("GET", "/api/v1/projects", nil)
		expect(t, code, 200, b)
		if items := b["items"].([]any); len(items) != 1 || items[0].(map[string]any)["archived"] != true {
			t.Fatalf("list = %v", items)
		}
		code, b = dev.do("GET", "/api/v1/projects/KYB/issues", nil)
		expect(t, code, 200, b) // still readable
		code, b = lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "More", "type": "task"})
		expect(t, code, 403, b)
		if d, _ := b["detail"].(string); d == "" || !contains(d, "archived") {
			t.Fatalf("detail = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "late"})
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S1"})
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "viewer"})
		expect(t, code, 409, b)

		code, b = dev.do("POST", "/api/v1/projects/KYB/restore", nil)
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/restore", nil)
		expect(t, code, 200, b)
		if b["archived"] != false {
			t.Fatalf("restore = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "More", "type": "task"})
		expect(t, code, 201, b)
	})
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// B10: a project can start from the Scrum or bug-tracking template.
func TestProjectTemplates(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		lead := c.signedIn("lead@x.uz")
		code, b := lead.do("POST", "/api/v1/projects", map[string]string{"key": "BUG", "name": "Bugs", "template": "bug-tracking"})
		expect(t, code, 201, b)
		code, b = lead.do("GET", "/api/v1/projects/BUG/workflow", nil)
		expect(t, code, 200, b)
		if b["initial"] != "open" || len(b["statuses"].([]any)) != 5 {
			t.Fatalf("workflow = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/projects", map[string]string{"key": "SCR", "name": "Scrum", "template": "scrum"})
		expect(t, code, 201, b)
		code, b = lead.do("GET", "/api/v1/projects/SCR/boards", nil)
		expect(t, code, 200, b)
		if items := b["items"].([]any); len(items) != 1 || items[0].(map[string]any)["kind"] != "scrum" {
			t.Fatalf("boards = %v", items)
		}
		code, b = lead.do("POST", "/api/v1/projects", map[string]string{"key": "BAD", "name": "Bad", "template": "waterfall"})
		expect(t, code, 422, b)
	})
}
