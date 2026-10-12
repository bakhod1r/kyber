package server_test

import (
	"fmt"
	"testing"
	"time"
)

// S47: the sprint report lists completed, not completed (carried over) and removed issues,
// and scope added after the start; completion can carry unfinished work into the next sprint.
func TestS47SprintReport(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("lead@x.uz")
		qa := anon.signedIn("qa@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		c.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "qa@x.uz", "role": "member"})
		for i, p := range []float64{3, 5, 2, 8} {
			_, is := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": fmt.Sprint("t", i+1), "type": "task"})
			c.do("PATCH", "/api/v1/issues/"+is["key"].(string), map[string]any{"version": is["version"], "estimate": p})
		}
		_, sp := c.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "Sprint 1"})
		_, next := c.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "Sprint 2"})
		sid, nid := sp["id"].(string), next["id"].(string)
		plan := func(key, sprint string) {
			_, is := c.do("GET", "/api/v1/issues/"+key, nil)
			code, b := c.do("PATCH", "/api/v1/issues/"+key, map[string]any{"version": is["version"], "sprint_id": sprint})
			expect(t, code, 200, b)
		}
		for _, k := range []string{"KYB-1", "KYB-2", "KYB-3"} {
			plan(k, sid)
		}
		code, rep := c.do("GET", "/api/v1/sprints/"+sid+"/report", nil)
		expect(t, code, 200, rep)
		if rep["committed_issues"] != float64(0) || len(rep["completed"].([]any)) != 0 {
			t.Fatalf("planned sprint report = %v", rep)
		}
		eventuallyJSON(t, c, "/api/v1/projects/KYB/reports/created-vs-resolved?days=1", func(b map[string]any) bool {
			return b["days"].([]any)[0].(map[string]any)["created"] == float64(4)
		})
		time.Sleep(30 * time.Millisecond)
		c.do("POST", "/api/v1/sprints/"+sid+"/start", nil)
		time.Sleep(30 * time.Millisecond)
		c.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		plan("KYB-4", sid) // added after the start
		plan("KYB-3", "")  // removed

		// Completing into a sprint of another state or project is refused.
		code, b := c.do("POST", "/api/v1/sprints/"+sid+"/complete", map[string]string{"move_to": sid})
		expect(t, code, 422, b)
		code, b = c.do("POST", "/api/v1/sprints/"+sid+"/complete", map[string]string{"move_to": nid})
		expect(t, code, 200, b)
		if b["completed"] != float64(1) || b["returned"] != float64(2) {
			t.Fatalf("completion = %v", b)
		}
		_, moved := c.do("GET", "/api/v1/issues/KYB-2", nil)
		if moved["sprint_id"] != nid {
			t.Fatalf("KYB-2 not carried to Sprint 2: %v", moved["sprint_id"])
		}

		keys := func(l any) string {
			s := ""
			for _, x := range l.([]any) {
				m := x.(map[string]any)
				s += fmt.Sprintf("%s:%v:%v ", m["key"], m["points"], m["added_after_start"])
			}
			return s
		}
		rep = eventuallyJSON(t, c, "/api/v1/sprints/"+sid+"/report", func(b map[string]any) bool {
			return len(b["not_completed"].([]any)) == 2 && len(b["removed"].([]any)) == 1
		})
		if keys(rep["completed"]) != "KYB-1:3:false " || keys(rep["not_completed"]) != "KYB-2:5:false KYB-4:8:true " ||
			keys(rep["removed"]) != "KYB-3:2:false " {
			t.Fatalf("lists: %v / %v / %v", rep["completed"], rep["not_completed"], rep["removed"])
		}
		if rep["committed_points"] != float64(10) || rep["committed_issues"] != float64(3) || rep["added_points"] != float64(8) ||
			rep["completed_points"] != float64(3) || rep["sprint"].(map[string]any)["state"] != "closed" {
			t.Fatalf("totals = %v", rep)
		}

		// A confidential issue is left out for those who may not see it.
		_, is := c.do("GET", "/api/v1/issues/KYB-1", nil)
		c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "security_level": "confidential"})
		_, mine := qa.do("GET", "/api/v1/sprints/"+sid+"/report", nil)
		if len(mine["completed"].([]any)) != 0 || mine["committed_points"] != float64(7) {
			t.Fatalf("qa sees the confidential issue: %v", mine)
		}
		if code, b := anon.signedIn("out@x.uz").do("GET", "/api/v1/sprints/"+sid+"/report", nil); code != 404 {
			t.Fatalf("outsider: %d %v", code, b)
		}
	})
}
