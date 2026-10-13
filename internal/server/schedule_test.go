package server_test

import "testing"

// Timeline: start and due dates on issues, validated and searchable with JQL.
func TestSchedule(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Parser", "type": "task"})
		if is["start_date"] != nil || is["due_date"] != nil {
			t.Fatalf("new issue is unscheduled: %v %v", is["start_date"], is["due_date"])
		}
		code, b := lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "start_date": "2026-10-20", "due_date": "2026-10-13"})
		expect(t, code, 422, b)
		code, is = lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "start_date": "2026-10-13", "due_date": "2026-10-20"})
		expect(t, code, 200, is)
		if is["start_date"] != "2026-10-13" || is["due_date"] != "2026-10-20" {
			t.Fatalf("schedule = %v %v", is["start_date"], is["due_date"])
		}
		code, res := lead.do("GET", "/api/v1/search?jql=due%20%3C%202026-10-31", nil)
		expect(t, code, 200, res)
		if keysOf(res) != "KYB-1" {
			t.Fatalf("search = %v", res)
		}
		code, is = lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "start_date": ""})
		expect(t, code, 200, is)
		if is["start_date"] != nil || is["due_date"] != "2026-10-20" {
			t.Fatalf("schedule = %v %v", is["start_date"], is["due_date"])
		}
	})
}
