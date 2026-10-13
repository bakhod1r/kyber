package server_test

import "testing"

// Worklog: estimates, logging work, the worklog list and taking work back.
func TestWorklog(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		vic := anon.signedIn("vic@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		_, is := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Parser", "type": "task"})
		code, b := dev.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "original_estimate": "nonsense"})
		expect(t, code, 422, b)
		code, is = dev.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "original_estimate": "1d 4h"})
		expect(t, code, 200, is)
		if tt := is["time_tracking"].(map[string]any); tt["original"] != "1d 4h" || tt["remaining"] != "1d 4h" || tt["original_seconds"] != float64(12*3600) {
			t.Fatalf("estimate = %v", tt)
		}
		const url = "/api/v1/issues/KYB-1/worklogs"
		code, w := dev.do("POST", url, map[string]string{"spent": "3h", "comment": "lexer", "started": "2026-10-14T09:00:00Z"})
		expect(t, code, 201, w)
		code, b = vic.do("POST", url, map[string]string{"spent": "1h"})
		expect(t, code, 403, b)
		code, b = dev.do("POST", url, map[string]string{"spent": "ages"})
		expect(t, code, 422, b)
		_, is = dev.do("GET", "/api/v1/issues/KYB-1", nil)
		if tt := is["time_tracking"].(map[string]any); tt["spent"] != "3h" || tt["remaining"] != "1d 1h" {
			t.Fatalf("after log = %v", tt)
		}
		list := eventuallyJSON(t, vic, url, func(b map[string]any) bool { return len(b["items"].([]any)) == 1 })
		first := list["items"].([]any)[0].(map[string]any)
		if first["author_name"] == "" || first["spent"] != "3h" || first["comment"] != "lexer" {
			t.Fatalf("worklog = %v", first)
		}
		code, b = vic.do("DELETE", "/api/v1/worklogs/"+w["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = dev.do("DELETE", "/api/v1/worklogs/"+w["id"].(string), nil)
		expect(t, code, 204, b)
		eventuallyJSON(t, dev, url, func(b map[string]any) bool { return len(b["items"].([]any)) == 0 })
		code, b = dev.do("DELETE", "/api/v1/worklogs/"+w["id"].(string), nil)
		expect(t, code, 404, b)
		_, is = dev.do("GET", "/api/v1/issues/KYB-1", nil)
		if tt := is["time_tracking"].(map[string]any); tt["spent"] != "" || tt["remaining"] != "1d 4h" {
			t.Fatalf("after delete = %v", tt)
		}
	})
}
