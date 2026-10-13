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
		if len(tools) != 5 {
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
	})
}
