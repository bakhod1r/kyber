package server_test

import (
	"strconv"
	"strings"
	"testing"
)

// Every change made through the API is in the project's audit log — including refused attempts.
func TestAuditLog(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		dev.do("PUT", "/api/v1/projects/KYB/workflow", map[string]any{"initial": "todo", "statuses": []any{}}) // refused: 403
		lead.do("GET", "/api/v1/projects/KYB/issues", nil)                                                     // reads are not audited
		_, sp := lead.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S1"})
		lead.do("POST", "/api/v1/sprints/"+sp["id"].(string)+"/start", map[string]any{})
		lead.do("GET", "/api/v1/projects/KYB/boards", nil) // creates the default board
		_, b := lead.do("POST", "/api/v1/projects/KYB/boards", map[string]any{"name": "Extra", "kind": "kanban"})
		lead.do("DELETE", "/api/v1/boards/"+b["id"].(string), nil) // resolved before the board is gone

		code, denied := dev.do("GET", "/api/v1/projects/KYB/audit", nil)
		expect(t, code, 403, denied)
		code, denied = anon.signedIn("alien@x.uz").do("GET", "/api/v1/projects/KYB/audit", nil)
		expect(t, code, 404, denied)
		code, log := lead.do("GET", "/api/v1/projects/KYB/audit", nil)
		expect(t, code, 200, log)
		var lines []string
		for _, it := range log["items"].([]any) {
			e := it.(map[string]any)
			if e["at"] == "" || e["actor_id"] == "" {
				t.Fatalf("entry = %v", e)
			}
			lines = append(lines, e["actor_name"].(string)+" "+e["action"].(string)+" "+e["target"].(string)+" "+strconv.Itoa(int(e["status"].(float64))))
		}
		want := []string{
			"Tester DELETE /api/v1/boards/{id} /api/v1/boards/" + b["id"].(string) + " 204",
			"Tester POST /api/v1/projects/{key}/boards /api/v1/projects/KYB/boards 201",
			"Tester POST /api/v1/sprints/{id}/start /api/v1/sprints/" + sp["id"].(string) + "/start 200",
			"Tester POST /api/v1/projects/{key}/sprints /api/v1/projects/KYB/sprints 201",
			"Tester PUT /api/v1/projects/{key}/workflow /api/v1/projects/KYB/workflow 403",
			"Tester POST /api/v1/issues/{issueKey}/transitions /api/v1/issues/KYB-1/transitions 200",
			"Tester POST /api/v1/projects/{key}/issues /api/v1/projects/KYB/issues 201",
			"Tester POST /api/v1/projects/{key}/members /api/v1/projects/KYB/members 204",
		}
		if strings.Join(lines, "\n") != strings.Join(want, "\n") {
			t.Fatalf("audit:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
		}
	})
}
