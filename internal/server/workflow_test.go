package server_test

import "testing"

// S43: each project configures its own workflow (statuses with Jira categories + transitions).
func TestS43ProjectWorkflows(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})

		code, wf := dev.do("GET", "/api/v1/projects/KYB/workflow", nil)
		expect(t, code, 200, wf)
		if wf["initial"] != "todo" || len(wf["statuses"].([]any)) != 3 || len(wf["transitions"].([]any)) != 0 {
			t.Fatalf("default workflow = %v", wf)
		}
		review := map[string]any{
			"initial": "open",
			"statuses": []map[string]string{
				{"id": "open", "name": "Open", "category": "todo"},
				{"id": "review", "name": "Code Review", "category": "in_progress"},
				{"id": "closed", "name": "Closed", "category": "done"},
			},
			"transitions": []map[string]string{{"from": "open", "to": "review"}, {"from": "review", "to": "closed"}},
		}
		code, b := dev.do("PUT", "/api/v1/projects/KYB/workflow", review)
		expect(t, code, 403, b)
		code, b = anon.signedIn("alien@x.uz").do("GET", "/api/v1/projects/KYB/workflow", nil)
		expect(t, code, 404, b)
		code, b = lead.do("PUT", "/api/v1/projects/KYB/workflow", map[string]any{"initial": "x", "statuses": []any{}})
		expect(t, code, 422, b)

		code, wf = lead.do("PUT", "/api/v1/projects/KYB/workflow", review)
		expect(t, code, 200, wf)
		code, is := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		expect(t, code, 201, is)
		if is["status"] != "open" {
			t.Fatalf("new issue status = %v", is["status"])
		}
		code, b = dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "closed"})
		expect(t, code, 409, b)
		code, b = dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "review"})
		expect(t, code, 200, b)

		// Removing a status that holds issues is refused.
		code, b = lead.do("PUT", "/api/v1/projects/KYB/workflow", map[string]any{"initial": "todo", "statuses": []map[string]string{
			{"id": "todo", "name": "To Do", "category": "todo"}, {"id": "done", "name": "Done", "category": "done"}}})
		expect(t, code, 409, b)
		if b["code"] != "WORKFLOW_STATUS_IN_USE" {
			t.Fatalf("problem = %v", b)
		}
	})
}
