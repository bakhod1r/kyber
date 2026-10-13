package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/mcp"
)

func server() *mcp.Server {
	s := mcp.New("kyber", "1.2.3")
	s.Add(mcp.Tool{Name: "echo", Description: "Echo the arguments.", InputSchema: json.RawMessage(`{"type":"object"}`),
		Call: func(_ context.Context, args json.RawMessage) (any, error) {
			var v map[string]any
			_ = json.Unmarshal(args, &v)
			return v, nil
		}})
	s.Add(mcp.Tool{Name: "list", Description: "A list.", InputSchema: json.RawMessage(`{"type":"object"}`),
		Call: func(context.Context, json.RawMessage) (any, error) { return []int{1, 2}, nil }})
	s.Add(mcp.Tool{Name: "fail", Description: "Always fails.", InputSchema: json.RawMessage(`{"type":"object"}`),
		Call: func(context.Context, json.RawMessage) (any, error) { return nil, errors.New("issue not found") }})
	s.Add(mcp.Tool{Name: "odd", Description: "Unencodable.", InputSchema: json.RawMessage(`{"type":"object"}`),
		Call: func(context.Context, json.RawMessage) (any, error) { return func() {}, nil }})
	return s
}

func post(t *testing.T, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	server().ServeHTTP(w, httptest.NewRequest(method, "/mcp", strings.NewReader(body)))
	return w
}

func TestServe(t *testing.T) {
	tests := []struct {
		name, method, body string
		status             int
		want               []string // substrings of the response body
	}{
		{"initialize negotiates the newest version", "POST", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2099-01-01"}}`, 200,
			[]string{`"protocolVersion":"2025-06-18"`, `"name":"kyber"`, `"version":"1.2.3"`, `"tools":{"listChanged":false}`}},
		{"initialize keeps an older supported version", "POST", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, 200,
			[]string{`"protocolVersion":"2025-03-26"`}},
		{"ping", "POST", `{"jsonrpc":"2.0","id":"a","method":"ping"}`, 200, []string{`"id":"a"`, `"result":{}`}},
		{"tools/list in order", "POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, 200,
			[]string{`{"tools":[{"name":"echo","description":"Echo the arguments.","inputSchema":{"type":"object"}},{"name":"list"`}},
		{"tools/call returns text and structured content", "POST", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"a":1}}}`, 200,
			[]string{`"text":"{\"a\":1}"`, `"structuredContent":{"a":1}`, `"isError":false`}},
		{"missing arguments are an empty object", "POST", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo"}}`, 200,
			[]string{`"text":"{}"`}},
		{"non-object results are text only", "POST", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list"}}`, 200,
			[]string{`"text":"[1,2]"`, `"isError":false}`}},
		{"tool errors are results", "POST", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"fail"}}`, 200,
			[]string{`"text":"issue not found"`, `"isError":true`}},
		{"unencodable results are tool errors", "POST", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"odd"}}`, 200,
			[]string{`unencodable result`, `"isError":true`}},
		{"unknown tool", "POST", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope"}}`, 200,
			[]string{`"code":-32602`, `unknown tool: nope`}},
		{"malformed call params", "POST", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":[1]}`, 200, []string{`"code":-32602`}},
		{"unknown method", "POST", `{"jsonrpc":"2.0","id":6,"method":"resources/list"}`, 200, []string{`"code":-32601`}},
		{"not JSON-RPC 2.0", "POST", `{"jsonrpc":"1.0","id":7,"method":"ping"}`, 200, []string{`"code":-32600`}},
		{"parse error", "POST", `{`, 200, []string{`"id":null`, `"code":-32700`}},
		{"oversized body", "POST", `"` + strings.Repeat("x", 1<<20+1) + `"`, 200, []string{`"code":-32700`}},
		{"notifications are accepted", "POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202, nil},
		{"GET has no stream", "GET", ``, 405, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := post(t, tt.method, tt.body)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.status, w.Body)
			}
			for _, s := range tt.want {
				if !strings.Contains(w.Body.String(), s) {
					t.Errorf("body %s lacks %s", w.Body, s)
				}
			}
			if tt.status == 200 && w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("content type = %q", w.Header().Get("Content-Type"))
			}
		})
	}
	if w := post(t, "GET", ""); w.Header().Get("Allow") != http.MethodPost {
		t.Errorf("Allow = %q", w.Header().Get("Allow"))
	}
}
