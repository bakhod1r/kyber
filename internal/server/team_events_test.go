package server_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
)

// meetingNote finds a meeting notification whose excerpt starts with prefix.
func meetingNote(title, prefix string) func(map[string]any) bool {
	return func(b map[string]any) bool {
		for _, it := range b["items"].([]any) {
			n := it.(map[string]any)
			if n["kind"] == "meeting" && n["issue_title"] == title && strings.HasPrefix(n["excerpt"].(string), prefix) {
				return true
			}
		}
		return false
	}
}

// Team events: a meeting for a project or the workspace invites and notifies every member,
// reminds them before it starts and takes their answers.
func TestTeamEvents(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		vic := anon.signedIn("vic@x.uz")
		boss := anon.signedIn("boss@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})

		start := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
		slot := func(title, scope string) map[string]any {
			return map[string]any{"title": title, "starts_at": start, "ends_at": start.Add(time.Hour), "scope": scope, "project_key": "KYB"}
		}
		code, b := vic.do("POST", "/api/v1/meetings", slot("Viewer event", "project"))
		expect(t, code, 403, b)
		code, m := lead.do("POST", "/api/v1/meetings", slot("Sprint review", "project"))
		expect(t, code, 201, m)
		if len(m["attendee_ids"].([]any)) != 3 || m["scope"] != "project" || m["project_key"] != "KYB" {
			t.Fatalf("meeting = %v", m)
		}
		eventuallyJSON(t, dev, "/api/v1/notifications", meetingNote("Sprint review", start.Format("Mon 2 Jan, 15:04"))) // scheduled
		// It starts within 15 minutes, so the reminder follows.
		eventuallyJSON(t, vic, "/api/v1/notifications", meetingNote("Sprint review", "Starts at"))

		id := m["id"].(string)
		code, m = dev.do("PUT", "/api/v1/meetings/"+id+"/response", map[string]string{"response": "going"})
		expect(t, code, 200, m)
		if m["responses"].(map[string]any)[m["organizer_id"].(string)] != nil || len(m["responses"].(map[string]any)) != 1 {
			t.Fatalf("responses = %v", m["responses"])
		}
		code, b = boss.do("PUT", "/api/v1/meetings/"+id+"/response", map[string]string{"response": "going"})
		expect(t, code, 404, b)
		code, b = dev.do("PUT", "/api/v1/meetings/"+id+"/response", map[string]string{"response": "perhaps"})
		expect(t, code, 422, b)

		// Workspace-wide: only configured administrators in single-tenant mode.
		code, b = lead.do("POST", "/api/v1/meetings", slot("All hands", "workspace"))
		expect(t, code, 403, b)
		code, w := boss.do("POST", "/api/v1/meetings", slot("All hands", "workspace"))
		expect(t, code, 201, w)
		if len(w["attendee_ids"].([]any)) != 4 { // the project's three members and the organizer
			t.Fatalf("all hands = %v", w["attendee_ids"])
		}

		code, b = lead.do("DELETE", "/api/v1/meetings/"+id, nil)
		expect(t, code, 204, b)
		eventuallyJSON(t, dev, "/api/v1/notifications", meetingNote("Sprint review", "Cancelled"))
	}, server.WithSiteAdmins("boss@x.uz"), server.WithReminderEvery(100*time.Millisecond))
}
