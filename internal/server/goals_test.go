package server_test

import "testing"

// Goals: a project's outcomes with progress measured by JQL over the caller's visible issues.
func TestGoals(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		for _, title := range []string{"Bug one", "Bug two"} {
			lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": title, "type": "bug"})
		}
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Task", "type": "task"})
		_, is := lead.do("GET", "/api/v1/issues/KYB-1", nil)
		lead.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]any{"to": "done", "version": is["version"]})

		const url = "/api/v1/projects/KYB/goals"
		code, b := lead.do("POST", url, map[string]string{"name": "Bugs", "jql": "type = bug ORDER BY key"})
		expect(t, code, 422, b)
		code, b = lead.do("POST", url, map[string]string{"name": "Bugs", "jql": "type ="})
		expect(t, code, 422, b)
		code, b = lead.do("POST", url, map[string]string{"name": "", "jql": ""})
		expect(t, code, 422, b)
		code, b = vic.do("POST", url, map[string]string{"name": "Mine"})
		expect(t, code, 403, b)
		code, g := lead.do("POST", url, map[string]string{"name": "Zero bugs", "target_date": "2026-12-31", "jql": "type = bug"})
		expect(t, code, 201, g)
		if g["done"] != float64(1) || g["total"] != float64(2) || g["percent"] != float64(50) || g["status"] != "on_track" {
			t.Fatalf("goal = %v", g)
		}

		code, list := vic.do("GET", url, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 {
			t.Fatalf("goals = %v", list)
		}
		code, b = out.do("GET", url, nil)
		expect(t, code, 404, b)

		gurl := "/api/v1/goals/" + g["id"].(string)
		code, b = lead.do("PUT", gurl, map[string]any{"version": 99, "name": "x", "status": "done"})
		expect(t, code, 409, b)
		code, b = vic.do("PUT", gurl, map[string]any{"version": g["version"], "name": "x", "status": "done"})
		expect(t, code, 403, b)
		code, g = lead.do("PUT", gurl, map[string]any{"version": g["version"], "name": "Zero bugs", "jql": "type = bug", "status": "at_risk"})
		expect(t, code, 200, g)
		if g["status"] != "at_risk" || g["target_date"] != nil {
			t.Fatalf("updated = %v", g)
		}
		code, b = out.do("DELETE", gurl, nil)
		expect(t, code, 404, b)
		code, b = lead.do("DELETE", gurl, nil)
		expect(t, code, 204, b)
	})
}
