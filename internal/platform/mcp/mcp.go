// Package mcp serves the Model Context Protocol (Streamable HTTP transport,
// JSON responses only) so AI assistants can call Kyber tools on a user's
// behalf. It knows nothing about Kyber's contexts: the composition root
// registers tools, and each tool runs with the caller's own permissions.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
)

// ProtocolVersion is the newest protocol revision this server speaks.
const ProtocolVersion = "2025-06-18"

// supported lists every revision a client may negotiate, newest first.
var supported = []string{ProtocolVersion, "2025-03-26"}

// maxBody bounds one JSON-RPC message.
const maxBody = 1 << 20

// JSON-RPC 2.0 error codes.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeNoMethod       = -32601
	codeInvalidParams  = -32602
)

// Tool is one callable capability. Call receives the raw arguments object; an
// error becomes a tool result flagged isError so the model can read and recover.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Call        func(ctx context.Context, args json.RawMessage) (any, error)
}

// Server dispatches JSON-RPC requests to registered tools.
type Server struct {
	name, version string
	tools         []Tool
}

// New returns a server that introduces itself with name and version.
func New(name, version string) *Server { return &Server{name: name, version: version} }

// Add registers a tool; tools are listed in registration order.
func (s *Server) Add(t Tool) { s.tools = append(s.tools, t) }

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// ServeHTTP accepts one JSON-RPC message per POST. Notifications and client
// responses are acknowledged with 202; requests get a JSON response.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		reply(w, response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "unreadable or oversized body"}})
		return
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		reply(w, response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, "invalid JSON-RPC message"}})
		return
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		// A notification (or a response to a server request): nothing to answer.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	res := response{JSONRPC: "2.0", ID: req.ID}
	if req.JSONRPC != "2.0" || req.Method == "" {
		res.Error = &rpcError{codeInvalidRequest, "not a JSON-RPC 2.0 request"}
	} else {
		res.Result, res.Error = s.dispatch(r.Context(), req)
	}
	reply(w, res)
}

func reply(w http.ResponseWriter, res response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil
	case "ping":
		return struct{}{}, nil
	case "tools/list":
		return s.list(), nil
	case "tools/call":
		return s.call(ctx, req.Params)
	}
	return nil, &rpcError{codeNoMethod, "method not found: " + req.Method}
}

func (s *Server) initialize(params json.RawMessage) any {
	var in struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &in)
	version := ProtocolVersion
	if slices.Contains(supported, in.ProtocolVersion) {
		version = in.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]string{"name": s.name, "version": s.version},
	}
}

type toolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func (s *Server) list() any {
	out := make([]toolInfo, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, toolInfo{t.Name, t.Description, t.InputSchema})
	}
	return map[string]any{"tools": out}
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content           []content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError"`
}

func (s *Server) call(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, &rpcError{codeInvalidParams, "params must be {name, arguments}"}
	}
	i := slices.IndexFunc(s.tools, func(t Tool) bool { return t.Name == in.Name })
	if i < 0 {
		return nil, &rpcError{codeInvalidParams, "unknown tool: " + in.Name}
	}
	if len(in.Arguments) == 0 || string(in.Arguments) == "null" {
		in.Arguments = json.RawMessage("{}")
	}
	out, err := s.tools[i].Call(ctx, in.Arguments)
	if err != nil {
		return callResult{Content: []content{{"text", err.Error()}}, IsError: true}, nil
	}
	text, err := json.Marshal(out)
	if err != nil {
		return callResult{Content: []content{{"text", errors.Join(errors.New("unencodable result"), err).Error()}}, IsError: true}, nil
	}
	return callResult{Content: []content{{"text", string(text)}}, StructuredContent: structured(out, text)}, nil
}

// structured returns the result as structured content when it is a JSON
// object, which is what the protocol requires of structuredContent.
func structured(out any, text []byte) any {
	if len(text) > 0 && text[0] == '{' {
		return out
	}
	return nil
}
