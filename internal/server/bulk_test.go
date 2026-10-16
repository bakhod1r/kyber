package server_test

import "testing"

// G9: change several issues at once; each one reports its own outcome.
func TestBulkChange(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		lead := c.signedIn("lead@x.uz")
		viewer := c.signedIn("vic@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		_, me := lead.do("GET", "/api/v1/me", nil)
		for range 3 {
			lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		}
		code, b := lead.do("POST", "/api/v1/projects/KYB/issues/bulk", map[string]any{
			"keys": []string{"KYB-1", "KYB-2", "KYB-99"}, "status": "in_progress", "assignee_id": me["id"], "add_labels": []string{"q4"},
		})
		expect(t, code, 200, b)
		results := b["results"].([]any)
		if len(results) != 3 || results[0].(map[string]any)["ok"] != true || results[1].(map[string]any)["ok"] != true {
			t.Fatalf("results = %v", results)
		}
		if r := results[2].(map[string]any); r["ok"] != false || r["code"] != "ISSUE_NOT_FOUND" || r["error"] == "" {
			t.Fatalf("missing issue = %v", r)
		}
		code, b = lead.do("GET", "/api/v1/issues/KYB-2", nil)
		expect(t, code, 200, b)
		if b["status"] != "in_progress" || b["assignee_id"] != me["id"] || b["labels"].([]any)[0] != "q4" {
			t.Fatalf("KYB-2 = %v", b)
		}
		code, b = viewer.do("POST", "/api/v1/projects/KYB/issues/bulk", map[string]any{"keys": []string{"KYB-3"}, "status": "done"})
		expect(t, code, 200, b)
		if r := b["results"].([]any)[0].(map[string]any); r["ok"] != false || r["code"] != "PROJECT_FORBIDDEN" {
			t.Fatalf("viewer = %v", r)
		}
		code, b = lead.do("POST", "/api/v1/projects/KYB/issues/bulk", map[string]any{"keys": []string{"KYB-3"}})
		expect(t, code, 422, b)
	})
}
