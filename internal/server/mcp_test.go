package server_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// MCP (ADR-0009): AI assistants call Kyber tools over JSON-RPC with the user's own token and permissions.
func TestMCP(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})

		rpc := func(c *client, method string, params any) map[string]any {
			t.Helper()
			code, b := c.do("POST", "/api/v1/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
			expect(t, code, 200, b)
			return b
		}
		// call returns the tool's JSON result, or its error text when the tool failed.
		call := func(c *client, tool string, args any) (map[string]any, string) {
			t.Helper()
			res := rpc(c, "tools/call", map[string]any{"name": tool, "arguments": args})["result"].(map[string]any)
			text := res["content"].([]any)[0].(map[string]any)["text"].(string)
			if res["isError"] == true {
				return nil, text
			}
			var out map[string]any
			if err := json.Unmarshal([]byte(text), &out); err != nil {
				t.Fatalf("%s: %v", text, err)
			}
			return out, ""
		}
		failed := func(c *client, tool string, args any) {
			t.Helper()
			if _, msg := call(c, tool, args); msg == "" {
				t.Fatalf("%s(%v) succeeded", tool, args)
			}
		}

		code, b := anon.do("POST", "/api/v1/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
		expect(t, code, 401, b)
		init := rpc(lead, "initialize", map[string]string{"protocolVersion": "2025-06-18"})["result"].(map[string]any)
		if init["protocolVersion"] != "2025-06-18" {
			t.Fatalf("initialize = %v", init)
		}
		tools := rpc(lead, "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
		if len(tools) != 11 {
			t.Fatalf("tools = %v", tools)
		}

		out, _ := call(lead, "list_projects", nil)
		if ps := out["projects"].([]any); len(ps) != 1 || ps[0].(map[string]any)["key"] != "KYB" {
			t.Fatalf("projects = %v", out)
		}
		is, msg := call(lead, "create_issue", map[string]string{"project": "KYB", "title": "Crash on login", "type": "bug", "description": "Stack trace"})
		if msg != "" || is["key"] != "KYB-1" || is["description"] != "Stack trace" {
			t.Fatalf("created = %v %s", is, msg)
		}
		if is, _ = call(lead, "create_issue", map[string]string{"project": "KYB", "title": "Plain", "type": "task"}); is["key"] != "KYB-2" {
			t.Fatalf("created = %v", is)
		}
		failed(lead, "create_issue", map[string]string{"project": "KYB", "title": "Long", "type": "task", "description": strings.Repeat("x", 20_001)})
		failed(vic, "create_issue", map[string]string{"project": "KYB", "title": "Nope", "type": "bug"})
		failed(lead, "create_issue", []int{1})

		got, _ := call(vic, "get_issue", map[string]string{"key": "KYB-1"})
		if got["title"] != "Crash on login" || got["type"] != "bug" {
			t.Fatalf("issue = %v", got)
		}
		failed(vic, "get_issue", map[string]string{"key": "NOPE-1"})
		failed(vic, "get_issue", "KYB-1")

		found, _ := call(vic, "search_issues", map[string]string{"jql": "type = bug"})
		if found["total"] != float64(1) || found["issues"].([]any)[0].(map[string]any)["key"] != "KYB-1" {
			t.Fatalf("search = %v", found)
		}
		failed(vic, "search_issues", map[string]string{"jql": "type ="})
		failed(vic, "search_issues", 7)

		c, _ := call(lead, "add_comment", map[string]string{"key": "KYB-1", "body": "Looking into it"})
		if c["body"] != "Looking into it" || c["issue"] != "KYB-1" {
			t.Fatalf("comment = %v", c)
		}
		failed(lead, "add_comment", map[string]string{"key": "KYB-1", "body": ""})
		failed(lead, "add_comment", false)

		cs, _ := call(vic, "get_comments", map[string]string{"key": "KYB-1"})
		if list := cs["comments"].([]any); len(list) != 1 || list[0].(map[string]any)["body"] != "Looking into it" {
			t.Fatalf("comments = %v", cs)
		}
		failed(vic, "get_comments", map[string]string{"key": "NOPE-1"})
		failed(vic, "get_comments", 1)

		moved, msg := call(lead, "transition_issue", map[string]string{"key": "KYB-1", "to": "in_progress"})
		if msg != "" || moved["status"] != "in_progress" {
			t.Fatalf("transition = %v %s", moved, msg)
		}
		failed(lead, "transition_issue", map[string]string{"key": "KYB-1", "to": "nowhere"})
		failed(lead, "transition_issue", 1)

		up, msg := call(lead, "update_issue", map[string]string{"key": "KYB-1", "priority": "high", "title": "Crash on login (iOS)"})
		if msg != "" || up["priority"] != "high" || up["title"] != "Crash on login (iOS)" || up["description"] != "Stack trace" {
			t.Fatalf("update = %v %s", up, msg)
		}
		failed(vic, "update_issue", map[string]string{"key": "KYB-1", "title": "x"})
		failed(lead, "update_issue", map[string]string{"key": "NOPE-1"})
		failed(lead, "update_issue", 1)

		ms, _ := call(vic, "list_members", map[string]string{"project": "KYB"})
		members := ms["members"].([]any)
		if len(members) != 2 {
			t.Fatalf("members = %v", ms)
		}
		failed(vic, "list_members", map[string]string{"project": "NOPE"})
		failed(vic, "list_members", 1)
		mine, msg := call(lead, "assign_issue", map[string]string{"key": "KYB-1", "assignee": "me"})
		if msg != "" || mine["assignee"] == "" {
			t.Fatalf("assign = %v %s", mine, msg)
		}
		if none, _ := call(lead, "assign_issue", map[string]string{"key": "KYB-1", "assignee": ""}); none["assignee"] != "" {
			t.Fatalf("unassign = %v", none)
		}
		failed(lead, "assign_issue", 1)

		lead.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "Sprint 1", "goal": "Ship"})
		sp, _ := call(vic, "list_sprints", map[string]string{"project": "KYB"})
		if list := sp["sprints"].([]any); len(list) != 1 || list[0].(map[string]any)["state"] != "planned" {
			t.Fatalf("sprints = %v", sp)
		}
		failed(vic, "list_sprints", map[string]string{"project": "NOPE"})
		failed(vic, "list_sprints", 1)
	})
}
