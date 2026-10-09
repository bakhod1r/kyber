package server_test

import (
	"strings"
	"testing"
	"time"
)

func TestS41S42PomodoroAndMeetings(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})

		// AC1 the lead books a meeting with dev in 20 minutes.
		now := time.Now().UTC()
		code, m := lead.do("POST", "/api/v1/meetings", map[string]any{"title": "Planning",
			"starts_at": now.Add(20 * time.Minute), "ends_at": now.Add(50 * time.Minute), "attendee_ids": []any{devMe["id"]}})
		expect(t, code, 201, m)
		code, b := lead.do("POST", "/api/v1/meetings", map[string]any{"title": "x", "starts_at": now, "ends_at": now.Add(time.Hour),
			"attendee_ids": []string{"00000000-0000-4000-8000-00000000dead"}})
		expect(t, code, 422, b)
		code, tt := dev.do("GET", "/api/v1/meetings", nil)
		if code != 200 || len(tt["items"].([]any)) != 1 {
			t.Fatalf("dev time table = %v", tt)
		}

		// AC2 a 25-minute Pomodoro would run into the meeting: refused, naming it.
		code, b = dev.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1", "minutes": 25})
		expect(t, code, 409, b)
		if b["code"] != "FOCUS_MEETING_CONFLICT" || !strings.Contains(b["detail"].(string), "Planning") {
			t.Fatalf("conflict = %v", b)
		}
		// A 15-minute one fits before it.
		code, s := dev.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1", "minutes": 15})
		expect(t, code, 201, s)
		if s["state"] != "running" || s["planned_minutes"] != float64(15) || s["remaining_seconds"].(float64) < 890 {
			t.Fatalf("session = %v", s)
		}
		code, b = dev.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1", "minutes": 15})
		expect(t, code, 409, b)

		// AC3 pause / resume / stop and the history on the issue.
		code, p := dev.do("POST", "/api/v1/focus/pause", nil)
		if code != 200 || p["state"] != "paused" {
			t.Fatalf("pause = %d %v", code, p)
		}
		code, b = dev.do("POST", "/api/v1/focus/pause", nil)
		expect(t, code, 409, b)
		code, r := dev.do("POST", "/api/v1/focus/resume", nil)
		if code != 200 || r["state"] != "running" {
			t.Fatalf("resume = %v", r)
		}
		code, cur := dev.do("GET", "/api/v1/focus", nil)
		if code != 200 || cur["session"] == nil {
			t.Fatalf("current = %v", cur)
		}
		code, st := dev.do("POST", "/api/v1/focus/stop", nil)
		if code != 200 || st["state"] != "interrupted" || st["reason"] != "stopped" || st["ended_at"] == nil {
			t.Fatalf("stop = %v", st)
		}
		code, none := dev.do("GET", "/api/v1/focus", nil)
		if code != 200 || none["session"] != nil {
			t.Fatalf("after stop = %v", none)
		}
		code, b = dev.do("POST", "/api/v1/focus/stop", nil)
		expect(t, code, 404, b)
		code, log := lead.do("GET", "/api/v1/issues/KYB-1/focus", nil)
		if code != 200 || len(log["items"].([]any)) != 1 {
			t.Fatalf("issue log = %v", log)
		}

		// AC4 only the organizer cancels; afterwards dev can focus for 25 minutes.
		code, b = dev.do("DELETE", "/api/v1/meetings/"+m["id"].(string), nil)
		expect(t, code, 403, b)
		code, _ = lead.do("DELETE", "/api/v1/meetings/"+m["id"].(string), nil)
		if code != 204 {
			t.Fatalf("cancel = %d", code)
		}
		code, s = dev.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1"})
		expect(t, code, 201, s)

		// AC5 outsiders: no focusing on, or reading, issues they cannot see.
		out := anon.signedIn("out@x.uz")
		code, b = out.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1"})
		expect(t, code, 404, b)
		code, b = out.do("GET", "/api/v1/issues/KYB-1/focus", nil)
		expect(t, code, 404, b)
		code, b = dev.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1", "minutes": 5})
		expect(t, code, 409, b) // still running the 25-minute one
	})
}
