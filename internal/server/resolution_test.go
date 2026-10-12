package server_test

import "testing"

// S48: Jira's resolution — set on entering a Done status, cleared when reopened, recorded in history.
func TestS48Resolution(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("lead@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, is := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Crash", "type": "bug"})
		if is["resolution"] != "" {
			t.Fatalf("new issue = %v", is)
		}
		const url = "/api/v1/issues/KYB-1/transitions"
		for _, bad := range []map[string]string{{"to": "in_progress", "resolution": "done"}, {"to": "done", "resolution": "fixed"}} {
			code, b := c.do("POST", url, bad)
			expect(t, code, 422, b)
		}
		code, b := c.do("POST", url, map[string]string{"to": "done", "resolution": "wont_do"})
		expect(t, code, 200, b)
		if b["resolution"] != "wont_do" {
			t.Fatalf("resolved = %v", b["resolution"])
		}
		_, b = c.do("POST", url, map[string]string{"to": "todo"})
		if b["resolution"] != "" {
			t.Fatalf("reopened = %v", b["resolution"])
		}
		_, b = c.do("POST", url, map[string]string{"to": "done"})
		if b["resolution"] != "done" {
			t.Fatalf("default resolution = %v", b["resolution"])
		}
		_, got := c.do("GET", "/api/v1/issues/KYB-1", nil)
		if got["resolution"] != "done" {
			t.Fatalf("stored = %v", got["resolution"])
		}
		// A resolved issue's resolution can be edited; an unresolved one cannot get one.
		code, got = c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": got["version"], "resolution": "duplicate"})
		expect(t, code, 200, got)
		if got["resolution"] != "duplicate" {
			t.Fatalf("edited = %v", got["resolution"])
		}
		_, open := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Open", "type": "task"})
		code, b = c.do("PATCH", "/api/v1/issues/KYB-2", map[string]any{"version": open["version"], "resolution": "done"})
		expect(t, code, 422, b)
		resolutions := func(h map[string]any) (n int) {
			for _, e := range h["items"].([]any) {
				if e.(map[string]any)["field"] == "resolution" {
					n++
				}
			}
			return n
		}
		eventuallyJSON(t, c, "/api/v1/issues/KYB-1/history", func(h map[string]any) bool { return resolutions(h) == 4 })
	})
}
