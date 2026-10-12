package server_test

import "testing"

// S44: Scrum and Kanban boards — columns map workflow statuses, with WIP limits; several boards per project.
func TestS44Boards(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})

		// First use creates the project's Kanban board from its workflow.
		code, list := dev.do("GET", "/api/v1/projects/KYB/boards", nil)
		expect(t, code, 200, list)
		items := list["items"].([]any)
		first := items[0].(map[string]any)
		if len(items) != 1 || first["name"] != "KYB board" || first["kind"] != "kanban" || len(first["columns"].([]any)) != 3 {
			t.Fatalf("default boards = %v", list)
		}

		scrum := map[string]any{"name": "Scrum", "kind": "scrum", "columns": []map[string]any{
			{"name": "Open", "statuses": []string{"todo"}},
			{"name": "Working", "statuses": []string{"in_progress", "done"}, "min": 0, "max": 2},
		}}
		code, b := dev.do("POST", "/api/v1/projects/KYB/boards", scrum)
		expect(t, code, 403, b)
		code, b = anon.signedIn("alien@x.uz").do("GET", "/api/v1/projects/KYB/boards", nil)
		expect(t, code, 404, b)
		code, b = lead.do("POST", "/api/v1/projects/KYB/boards", map[string]any{"name": "Bad", "kind": "scrum",
			"columns": []map[string]any{{"name": "X", "statuses": []string{"ghost"}}}})
		expect(t, code, 422, b)

		code, b = lead.do("POST", "/api/v1/projects/KYB/boards", scrum)
		expect(t, code, 201, b)
		id := b["id"].(string)
		code, got := dev.do("GET", "/api/v1/boards/"+id, nil)
		expect(t, code, 200, got)
		if got["name"] != "Scrum" || got["version"] != float64(1) {
			t.Fatalf("board = %v", got)
		}

		scrum["name"], scrum["version"] = "Team Scrum", 1
		code, b = dev.do("PUT", "/api/v1/boards/"+id, scrum)
		expect(t, code, 403, b)
		code, b = lead.do("PUT", "/api/v1/boards/"+id, scrum)
		expect(t, code, 200, b)
		code, b = lead.do("PUT", "/api/v1/boards/"+id, scrum) // stale version
		expect(t, code, 409, b)
		if b["code"] != "BOARD_CONFLICT" {
			t.Fatalf("problem = %v", b)
		}

		code, b = lead.do("DELETE", "/api/v1/boards/"+first["id"].(string), nil)
		expect(t, code, 204, b)
		code, b = lead.do("DELETE", "/api/v1/boards/"+id, nil)
		expect(t, code, 409, b)
		if b["code"] != "BOARD_LAST" {
			t.Fatalf("problem = %v", b)
		}
		code, b = dev.do("GET", "/api/v1/boards/not-a-board", nil)
		expect(t, code, 404, b)
	})
}
