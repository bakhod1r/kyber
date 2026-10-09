package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

// Acceptance tests for docs/backlog/sprint-01.md. Each subtest names its story and AC.

type client struct {
	t   *testing.T
	srv *httptest.Server
}

func newClient(t *testing.T) *client {
	t.Helper()
	srv := httptest.NewServer(server.NewInMemory(slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func expect(t *testing.T, got, want int, body map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d (body %v)", got, want, body)
	}
}

func TestHealthz(t *testing.T) {
	c := newClient(t)
	code, body := c.do("GET", "/healthz", nil)
	expect(t, code, 200, body)
}

func TestS1CreateProject(t *testing.T) {
	c := newClient(t)

	t.Run("AC1 created", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		expect(t, code, 201, body)
		if body["key"] != "KYB" || body["name"] != "Kyber" || body["id"] == "" {
			t.Fatalf("body = %v", body)
		}
	})
	t.Run("AC2 invalid key", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects", map[string]string{"key": "k", "name": "x"})
		expect(t, code, 422, body)
		if body["error"] == nil {
			t.Fatalf("error message missing: %v", body)
		}
	})
	t.Run("AC3 empty name", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects", map[string]string{"key": "ABC", "name": " "})
		expect(t, code, 422, body)
	})
	t.Run("AC4 duplicate", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Again"})
		expect(t, code, 409, body)
	})
	t.Run("AC5 get and list", func(t *testing.T) {
		code, body := c.do("GET", "/api/v1/projects/KYB", nil)
		expect(t, code, 200, body)
		code, body = c.do("GET", "/api/v1/projects/NOPE", nil)
		expect(t, code, 404, body)
		code, body = c.do("GET", "/api/v1/projects", nil)
		expect(t, code, 200, body)
		if items := body["items"].([]any); len(items) != 1 {
			t.Fatalf("items = %v", items)
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		req, _ := http.NewRequest("POST", c.srv.URL+"/api/v1/projects", bytes.NewBufferString("{"))
		res, _ := http.DefaultClient.Do(req)
		res.Body.Close()
		expect(t, res.StatusCode, 400, nil)
	})
}

func TestS2toS5IssueLifecycle(t *testing.T) {
	c := newClient(t)
	c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})

	t.Run("S2 AC1+AC2 create with sequential keys and todo status", func(t *testing.T) {
		for i, want := range []string{"KYB-1", "KYB-2", "KYB-3"} {
			code, body := c.do("POST", "/api/v1/projects/KYB/issues",
				map[string]string{"title": "Issue " + want, "type": []string{"task", "bug", "story"}[i]})
			expect(t, code, 201, body)
			if body["key"] != want || body["status"] != "todo" {
				t.Fatalf("body = %v", body)
			}
		}
	})
	t.Run("S2 AC3 validation", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": " ", "type": "task"})
		expect(t, code, 422, body)
		code, body = c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "x", "type": "feature"})
		expect(t, code, 422, body)
	})
	t.Run("S2 AC4 unknown project", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/projects/NOPE/issues", map[string]string{"title": "x", "type": "task"})
		expect(t, code, 404, body)
	})
	t.Run("S3 view", func(t *testing.T) {
		code, body := c.do("GET", "/api/v1/issues/KYB-1", nil)
		expect(t, code, 200, body)
		if body["title"] != "Issue KYB-1" || body["type"] != "task" {
			t.Fatalf("body = %v", body)
		}
		code, body = c.do("GET", "/api/v1/issues/garbage", nil)
		expect(t, code, 400, body)
		code, body = c.do("GET", "/api/v1/issues/KYB-99", nil)
		expect(t, code, 404, body)
	})
	t.Run("S4 transitions", func(t *testing.T) {
		code, body := c.do("POST", "/api/v1/issues/KYB-2/transitions", map[string]string{"to": "done"})
		expect(t, code, 409, body)
		code, body = c.do("POST", "/api/v1/issues/KYB-2/transitions", map[string]string{"to": "in_progress"})
		expect(t, code, 200, body)
		if body["status"] != "in_progress" {
			t.Fatalf("body = %v", body)
		}
	})
	t.Run("S5 list and filter", func(t *testing.T) {
		code, body := c.do("GET", "/api/v1/projects/KYB/issues", nil)
		expect(t, code, 200, body)
		items := body["items"].([]any)
		if len(items) != 3 || items[0].(map[string]any)["key"] != "KYB-1" {
			t.Fatalf("items = %v", items)
		}
		code, body = c.do("GET", "/api/v1/projects/KYB/issues?status=in_progress", nil)
		expect(t, code, 200, body)
		items = body["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["key"] != "KYB-2" {
			t.Fatalf("filtered = %v", items)
		}
	})
}
