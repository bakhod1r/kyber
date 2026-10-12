package server_test

import (
	"strings"
	"testing"
	"time"
)

// Every action on an issue is kept: who, when, which field, old → new (Jira's History tab).
func TestIssueHistory(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		lead.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
		_, is = lead.do("GET", "/api/v1/issues/KYB-1", nil)
		lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "title": "Login page", "priority": "high", "estimate": 3})
		lead.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "Ready for review"})

		var lines []string
		deadline := time.Now().Add(5 * time.Second) // the relay projects asynchronously
		for time.Now().Before(deadline) {
			code, h := lead.do("GET", "/api/v1/issues/KYB-1/history", nil)
			expect(t, code, 200, h)
			lines = lines[:0]
			for _, it := range h["items"].([]any) {
				e := it.(map[string]any)
				if e["by_name"] != "Tester" || e["at"] == "" {
					t.Fatalf("entry = %v", e)
				}
				lines = append(lines, e["field"].(string)+":"+e["from"].(string)+">"+e["to"].(string))
			}
			if len(lines) == 6 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		want := "created:>Login,status:todo>in_progress,title:Login>Login page,priority:medium>high,estimate:>3,comment:>Ready for review"
		if strings.Join(lines, ",") != want {
			t.Fatalf("history = %s\nwant      %s", strings.Join(lines, ","), want)
		}
		code, b := anon.signedIn("alien@x.uz").do("GET", "/api/v1/issues/KYB-1/history", nil)
		expect(t, code, 404, b)
	})
}
