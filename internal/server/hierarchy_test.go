package server_test

import "testing"

// S45: epic → story/task/bug → sub-task, with epic progress.
func TestS45Hierarchy(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Checkout", "type": "epic"})
		code, story := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Pay", "type": "story", "parent_key": "KYB-1"})
		expect(t, code, 201, story)
		if story["parent_key"] != "KYB-1" {
			t.Fatalf("story = %v", story)
		}
		code, b := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Orphan", "type": "subtask"})
		expect(t, code, 422, b)
		if b["code"] != "ISSUE_INVALID_PARENT" {
			t.Fatalf("problem = %v", b)
		}
		code, sub := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Form", "type": "subtask", "parent_key": "KYB-2"})
		expect(t, code, 201, sub)
		_, task := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Refunds", "type": "task"})
		code, task = lead.do("PATCH", "/api/v1/issues/KYB-4", map[string]any{"version": task["version"], "parent_key": "KYB-1"})
		expect(t, code, 200, task)
		lead.do("POST", "/api/v1/issues/KYB-2/transitions", map[string]string{"to": "done"})

		code, kids := lead.do("GET", "/api/v1/issues/KYB-1/children", nil)
		expect(t, code, 200, kids)
		if len(kids["items"].([]any)) != 2 || kids["done"] != float64(1) || kids["total"] != float64(2) {
			t.Fatalf("children = %v", kids)
		}
		code, task = lead.do("PATCH", "/api/v1/issues/KYB-4", map[string]any{"version": task["version"], "parent_key": nil})
		expect(t, code, 200, task)
		if task["parent_key"] != nil {
			t.Fatalf("detached = %v", task)
		}
	})
}
