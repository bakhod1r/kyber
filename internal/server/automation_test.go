package server_test

import (
	"strings"
	"testing"
)

// G8: a project admin's rule runs when a person moves a bug to In Review: the reporter gets it back, a label
// and a comment are added. Rules never react to their own changes; failures show up in the runs.
func TestAutomationRules(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		lead := c.signedIn("lead@x.uz")
		dev := c.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		_, me := lead.do("GET", "/api/v1/me", nil)

		rule := map[string]any{"name": "Review bugs", "trigger": "issue.transitioned", "to_status": "in_progress", "condition": "type = bug",
			"actions": []map[string]string{{"kind": "assign", "value": "reporter"}, {"kind": "add_label", "value": "triage"}, {"kind": "comment", "value": "Picked up automatically"}}}
		code, b := dev.do("POST", "/api/v1/projects/KYB/automation/rules", rule)
		expect(t, code, 403, b)
		bad := map[string]any{"name": "Broken", "trigger": "issue.created", "condition": "type = (", "actions": rule["actions"]}
		code, b = lead.do("POST", "/api/v1/projects/KYB/automation/rules", bad)
		expect(t, code, 422, b)
		code, created := lead.do("POST", "/api/v1/projects/KYB/automation/rules", rule)
		expect(t, code, 201, created)
		if created["enabled"] != true || created["owner_id"] != me["id"] {
			t.Fatalf("rule = %v", created)
		}

		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Crash", "type": "bug"})
		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Feature", "type": "story"})
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
		dev.do("POST", "/api/v1/issues/KYB-2/transitions", map[string]string{"to": "in_progress"}) // not a bug

		_, devMe := dev.do("GET", "/api/v1/me", nil)
		// The actions run in order, so wait for the last field one of them sets, not only the first.
		eventuallyJSON(t, lead, "/api/v1/issues/KYB-1", func(m map[string]any) bool {
			labels, _ := m["labels"].([]any)
			return m["assignee_id"] == devMe["id"] && len(labels) == 1 && labels[0] == "triage"
		})
		eventuallyJSON(t, lead, "/api/v1/issues/KYB-1/comments", func(m map[string]any) bool {
			items, _ := m["items"].([]any)
			return len(items) == 1 && strings.Contains(items[0].(map[string]any)["body"].(string), "Picked up automatically")
		})
		code, b = lead.do("GET", "/api/v1/issues/KYB-2", nil)
		expect(t, code, 200, b)
		if b["assignee_id"] != nil && b["assignee_id"] != "" {
			t.Fatalf("the story matched the bug condition: %v", b)
		}

		code, b = dev.do("GET", "/api/v1/projects/KYB/automation/runs", nil)
		expect(t, code, 200, b)
		runs := b["items"].([]any)
		if len(runs) != 1 || runs[0].(map[string]any)["ok"] != true || runs[0].(map[string]any)["issue_key"] != "KYB-1" {
			t.Fatalf("runs = %v", runs)
		}

		// Disable: nothing more happens.
		id := created["id"].(string)
		off := map[string]any{"version": created["version"], "enabled": false, "name": rule["name"], "trigger": rule["trigger"],
			"to_status": rule["to_status"], "condition": rule["condition"], "actions": rule["actions"]}
		code, b = lead.do("PUT", "/api/v1/automation/rules/"+id, off)
		expect(t, code, 200, b)
		if b["enabled"] != false {
			t.Fatalf("disabled = %v", b)
		}
		code, b = lead.do("GET", "/api/v1/projects/KYB/automation/rules", nil)
		expect(t, code, 200, b)
		if items := b["items"].([]any); len(items) != 1 {
			t.Fatalf("rules = %v", items)
		}
		code, _ = lead.do("DELETE", "/api/v1/automation/rules/"+id, nil)
		expect(t, code, 204, nil)
	})
}
