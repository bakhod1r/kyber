package server_test

import "testing"

// S48: a Kanban board can keep a Kanban backlog (its first column listed apart).
func TestS48KanbanBacklog(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		code, b := lead.do("POST", "/api/v1/projects/KYB/boards", map[string]any{"name": "Flow", "kind": "kanban", "kanban_backlog": true})
		expect(t, code, 201, b)
		if b["kanban_backlog"] != true {
			t.Fatalf("created = %v", b)
		}
		id := b["id"].(string)
		code, b = lead.do("PUT", "/api/v1/boards/"+id, map[string]any{"version": b["version"], "name": "Flow", "kind": "scrum", "kanban_backlog": true})
		expect(t, code, 422, b)
		_, got := lead.do("GET", "/api/v1/boards/"+id, nil)
		code, b = lead.do("PUT", "/api/v1/boards/"+id, map[string]any{"version": got["version"], "name": "Flow", "kind": "kanban", "columns": got["columns"]})
		expect(t, code, 200, b)
		if b["kanban_backlog"] != false {
			t.Fatalf("turned off = %v", b)
		}
	})
}
