package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/server"
)

// Acceptance tests for docs/backlog/sprint-01.md. Each subtest names its story and AC.

type client struct {
	t     *testing.T
	srv   *httptest.Server
	token string
}

// backends returns the server factories under test: memory always, Postgres when configured.
func backends(t *testing.T) map[string]func(t *testing.T) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := map[string]func(t *testing.T) http.Handler{
		"memory": func(t *testing.T) http.Handler {
			return server.NewInMemory(log, server.WithContext(t.Context()), server.WithRelayEvery(20*time.Millisecond))
		},
	}
	if os.Getenv("KYBER_TEST_DATABASE_URL") != "" {
		b["postgres"] = func(t *testing.T) http.Handler {
			return server.NewPostgres(log, dbtest.New(t), false, server.WithContext(t.Context()), server.WithRelayEvery(20*time.Millisecond))
		}
		b["postgres+redis"] = func(t *testing.T) http.Handler {
			rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
			return server.NewPostgres(log, dbtest.New(t), false, server.WithRedis(rdb),
				server.WithContext(t.Context()), server.WithRelayEvery(20*time.Millisecond))
		}
	}
	return b
}

// forEachBackend runs fn with an anonymous client per backend.
func forEachBackend(t *testing.T, fn func(t *testing.T, c *client)) {
	for name, mk := range backends(t) {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(mk(t))
			t.Cleanup(srv.Close)
			fn(t, &client{t: t, srv: srv})
		})
	}
}

// signedIn registers and logs in a user, returning a client sending a Bearer token.
func (c *client) signedIn(email string) *client {
	c.t.Helper()
	c.do("POST", "/api/v1/auth/signup", map[string]string{"email": email, "name": "Tester", "password": "long enough pw"})
	code, body := c.do("POST", "/api/v1/auth/login", map[string]string{"email": email, "password": "long enough pw"})
	expect(c.t, code, 200, body)
	return &client{t: c.t, srv: c.srv, token: body["token"].(string)}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	res := c.raw(method, path, body, nil)
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (c *client) raw(method, path string, body any, cookie *http.Cookie) *http.Response {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	spec.check(c.t, req, res)
	return res
}

func expect(t *testing.T, got, want int, body map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d (body %v)", got, want, body)
	}
}

func TestOps(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		code, body := c.do("GET", "/healthz", nil)
		expect(t, code, 200, body)
		code, body = c.do("GET", "/readyz", nil)
		expect(t, code, 200, body)
		res := c.raw("GET", "/metrics", nil, nil)
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(string(b), `kyber_http_requests_total{method="GET",status="200"}`) {
			t.Fatalf("metrics = %d %s", res.StatusCode, b)
		}
	})
}

func TestS9Auth(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		t.Run("AC1 signup", func(t *testing.T) {
			code, body := c.do("POST", "/api/v1/auth/signup", map[string]string{"email": " Ali@X.uz", "name": "Ali", "password": "long enough pw"})
			expect(t, code, 201, body)
			if body["email"] != "ali@x.uz" || body["password_hash"] != nil {
				t.Fatalf("body = %v", body)
			}
			code, body = c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "ali@x.uz", "name": "Ali", "password": "long enough pw"})
			expect(t, code, 409, body)
			code, body = c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "b@x.uz", "name": "B", "password": "short"})
			expect(t, code, 422, body)
			code, body = c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "nope", "name": "B", "password": "long enough pw"})
			expect(t, code, 422, body)
		})
		t.Run("AC2 login sets secure cookie and returns token", func(t *testing.T) {
			res := c.raw("POST", "/api/v1/auth/login", map[string]string{"email": "ali@x.uz", "password": "long enough pw"}, nil)
			res.Body.Close()
			expect(t, res.StatusCode, 200, nil)
			var ck *http.Cookie
			for _, x := range res.Cookies() {
				if x.Name == "kyber_session" {
					ck = x
				}
			}
			if ck == nil || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Value == "" {
				t.Fatalf("cookie = %+v", ck)
			}
			t.Run("AC4 me via cookie, logout invalidates", func(t *testing.T) {
				res := c.raw("GET", "/api/v1/me", nil, ck)
				var me map[string]any
				_ = json.NewDecoder(res.Body).Decode(&me)
				res.Body.Close()
				expect(t, res.StatusCode, 200, me)
				if me["email"] != "ali@x.uz" {
					t.Fatalf("me = %v", me)
				}
				res = c.raw("POST", "/api/v1/auth/logout", nil, ck)
				res.Body.Close()
				expect(t, res.StatusCode, 204, nil)
				res = c.raw("GET", "/api/v1/me", nil, ck)
				res.Body.Close()
				expect(t, res.StatusCode, 401, nil)
			})
		})
		t.Run("AC3 no user enumeration", func(t *testing.T) {
			code1, b1 := c.do("POST", "/api/v1/auth/login", map[string]string{"email": "ali@x.uz", "password": "wrong password"})
			code2, b2 := c.do("POST", "/api/v1/auth/login", map[string]string{"email": "ghost@x.uz", "password": "wrong password"})
			expect(t, code1, 401, b1)
			expect(t, code2, 401, b2)
			if b1["detail"] != b2["detail"] || b1["code"] != b2["code"] {
				t.Fatalf("messages differ: %v vs %v", b1, b2)
			}
		})
	})
}

func TestS10APIRequiresAuth(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		for _, r := range [][2]string{
			{"GET", "/api/v1/projects"}, {"POST", "/api/v1/projects"}, {"GET", "/api/v1/me"},
			{"GET", "/api/v1/issues/KYB-1"}, {"POST", "/api/v1/auth/logout"},
		} {
			code, body := c.do(r[0], r[1], map[string]string{})
			expect(t, code, 401, body)
		}
		bad := &client{t: t, srv: c.srv, token: "forged"}
		code, body := bad.do("GET", "/api/v1/projects", nil)
		expect(t, code, 401, body)
		code, body = c.signedIn("bearer@x.uz").do("GET", "/api/v1/projects", nil)
		expect(t, code, 200, body)
	})
}

func TestS1CreateProject(t *testing.T) {
	forEachBackend(t, testS1)
}

func testS1(t *testing.T, anon *client) {
	c := anon.signedIn("s1@x.uz")

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
		if body["code"] != "VALIDATION_FAILED" || body["detail"] == nil {
			t.Fatalf("problem body = %v", body)
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
		req.Header.Set("Authorization", "Bearer "+c.token)
		res, _ := http.DefaultClient.Do(req)
		res.Body.Close()
		expect(t, res.StatusCode, 400, nil)
	})
}

func TestS2toS5IssueLifecycle(t *testing.T) {
	forEachBackend(t, testIssues)
}

func testIssues(t *testing.T, anon *client) {
	c := anon.signedIn("issues@x.uz")
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

func TestS11Membership(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret", "type": "task"})

		t.Run("AC2 outsiders see nothing", func(t *testing.T) {
			for _, p := range []string{"/api/v1/projects/KYB", "/api/v1/projects/KYB/issues", "/api/v1/issues/KYB-1", "/api/v1/issues/KYB-999", "/api/v1/projects/KYB/members"} {
				code, body := bob.do("GET", p, nil)
				expect(t, code, 404, body)
			}
			code, body := bob.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "x", "type": "task"})
			expect(t, code, 404, body)
			_, body = bob.do("GET", "/api/v1/projects", nil)
			if items := body["items"].([]any); len(items) != 0 {
				t.Fatalf("bob sees %v", items)
			}
		})
		t.Run("AC4 admin manages members", func(t *testing.T) {
			code, body := alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "viewer"})
			expect(t, code, 204, body)
			code, body = alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "ghost@x.uz", "role": "viewer"})
			expect(t, code, 404, body)
			code, body = alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "owner"})
			expect(t, code, 422, body)
			code, body = bob.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "admin"})
			expect(t, code, 403, body)
		})
		t.Run("AC3 viewer is read-only", func(t *testing.T) {
			code, body := bob.do("GET", "/api/v1/issues/KYB-1", nil)
			expect(t, code, 200, body)
			code, body = bob.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "x", "type": "task"})
			expect(t, code, 403, body)
			code, body = bob.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
			expect(t, code, 403, body)
		})
		t.Run("AC4 re-adding updates role", func(t *testing.T) {
			alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "member"})
			code, body := bob.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
			expect(t, code, 200, body)
		})
		t.Run("AC5 list members", func(t *testing.T) {
			code, body := bob.do("GET", "/api/v1/projects/KYB/members", nil)
			expect(t, code, 200, body)
			items := body["items"].([]any)
			if len(items) != 2 {
				t.Fatalf("members = %v", items)
			}
			roles := map[string]string{}
			for _, it := range items {
				m := it.(map[string]any)
				roles[m["email"].(string)] = m["role"].(string)
			}
			if roles["alice@x.uz"] != "admin" || roles["bob@x.uz"] != "member" {
				t.Fatalf("roles = %v", roles)
			}
		})
		t.Run("AC6 last admin stays", func(t *testing.T) {
			code, body := alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "alice@x.uz", "role": "member"})
			expect(t, code, 409, body)
		})
	})
}

func TestS12LoginThrottling(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "t@x.uz", "name": "T", "password": "long enough pw"})
		for range 10 { // KYB-S12 revised: 10 attempts per email+IP per 15 minutes
			code, body := c.do("POST", "/api/v1/auth/login", map[string]string{"email": "t@x.uz", "password": "wrong password"})
			expect(t, code, 401, body)
		}
		res := c.raw("POST", "/api/v1/auth/login", map[string]string{"email": "t@x.uz", "password": "long enough pw"}, nil)
		res.Body.Close()
		expect(t, res.StatusCode, 429, nil)
		if ra := res.Header.Get("Retry-After"); ra == "" || ra == "0" {
			t.Fatalf("Retry-After = %q", ra)
		}
	})
}

func TestS13CSRFGuard(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		anon.signedIn("csrf@x.uz") // creates the account
		res := anon.raw("POST", "/api/v1/auth/login", map[string]string{"email": "csrf@x.uz", "password": "long enough pw"}, nil)
		res.Body.Close()
		var ck *http.Cookie
		for _, x := range res.Cookies() {
			if x.Name == "kyber_session" {
				ck = x
			}
		}
		post := func(contentType string, withCookie bool, bearer string) int {
			req, _ := http.NewRequest("POST", anon.srv.URL+"/api/v1/projects", strings.NewReader(`{"key":"CS","name":"x"}`))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}
			if withCookie {
				req.AddCookie(ck)
			}
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			}
			r, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			return r.StatusCode
		}
		if got := post("text/plain", true, ""); got != 403 {
			t.Fatalf("cookie + text/plain = %d, want 403", got)
		}
		if got := post("application/x-www-form-urlencoded", true, ""); got != 403 {
			t.Fatalf("cookie + form = %d, want 403", got)
		}
		if got := post("application/json; charset=utf-8", true, ""); got != 201 {
			t.Fatalf("cookie + json = %d, want 201", got)
		}
		bearer := anon.signedIn("csrf@x.uz").token
		if got := post("", false, bearer); got != 409 { // project CS exists; bearer is not CSRF-checked
			t.Fatalf("bearer without content-type = %d, want 409", got)
		}
	})
}

func TestS14UIServed(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		for _, p := range []string{"/", "/login", "/projects/KYB"} {
			res := c.raw("GET", p, nil, nil)
			res.Body.Close()
			// 200 with a built UI, 503 with instructions when web/dist is empty; never 401/404.
			if res.StatusCode != 200 && res.StatusCode != 503 {
				t.Fatalf("GET %s = %d", p, res.StatusCode)
			}
		}
		code, body := c.do("GET", "/api/v1/unknown", nil)
		expect(t, code, 401, body) // API namespace never falls through to the UI
	})
}

func TestS15IssueDetails(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		carol := anon.signedIn("carol@x.uz")
		_, bobMe := bob.do("GET", "/api/v1/me", nil)
		_, carolMe := carol.do("GET", "/api/v1/me", nil)
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "viewer"})
		_, created := alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})

		t.Run("AC1 defaults", func(t *testing.T) {
			if created["priority"] != "medium" || created["description"] != "" || created["assignee_id"] != nil || created["version"] != float64(1) {
				t.Fatalf("created = %v", created)
			}
		})
		var version any = created["version"]
		t.Run("AC2 edit and assign", func(t *testing.T) {
			code, body := alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{
				"version": version, "title": "Login v2", "description": "Steps\n1. open", "priority": "high", "assignee_id": bobMe["id"],
			})
			expect(t, code, 200, body)
			if body["title"] != "Login v2" || body["priority"] != "high" || body["assignee_id"] != bobMe["id"] || body["version"] != float64(2) {
				t.Fatalf("body = %v", body)
			}
			version = body["version"]
			code, body = alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": version, "assignee_id": nil})
			expect(t, code, 200, body)
			if body["assignee_id"] != nil || body["title"] != "Login v2" {
				t.Fatalf("unassign body = %v", body)
			}
			version = body["version"]
		})
		t.Run("AC3 conflicts and validation", func(t *testing.T) {
			code, body := alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": 1, "title": "stale"})
			expect(t, code, 409, body)
			for _, patch := range []map[string]any{
				{"version": version, "priority": "urgent"},
				{"version": version, "title": "  "},
				{"version": version, "description": strings.Repeat("x", 20_001)},
			} {
				code, body := alice.do("PATCH", "/api/v1/issues/KYB-1", patch)
				expect(t, code, 422, body)
			}
			code, body = alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"title": "no version"})
			expect(t, code, 422, body)
		})
		t.Run("AC4 permissions", func(t *testing.T) {
			code, body := alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": version, "assignee_id": carolMe["id"]})
			expect(t, code, 422, body)
			code, body = bob.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": version, "title": "x"})
			expect(t, code, 403, body)
			code, body = carol.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": version, "title": "x"})
			expect(t, code, 404, body)
		})
	})
}

func TestS16Comments(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		carol := anon.signedIn("carol@x.uz")
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "viewer"})
		alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})

		code, body := alice.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "  First!  "})
		expect(t, code, 201, body)
		if body["body"] != "First!" || body["author_name"] != "Tester" || body["id"] == "" || body["created_at"] == "" {
			t.Fatalf("comment = %v", body)
		}
		alice.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "Second"})

		code, body = bob.do("GET", "/api/v1/issues/KYB-1/comments", nil)
		expect(t, code, 200, body)
		items := body["items"].([]any)
		if len(items) != 2 || items[0].(map[string]any)["body"] != "First!" || items[1].(map[string]any)["body"] != "Second" {
			t.Fatalf("items = %v", items)
		}
		code, body = alice.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "   "})
		expect(t, code, 422, body)
		code, body = bob.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "hi"})
		expect(t, code, 403, body)
		code, body = carol.do("GET", "/api/v1/issues/KYB-1/comments", nil)
		expect(t, code, 404, body)
	})
}

func TestProblemDetails(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("problem@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})

		res := c.raw("GET", "/api/v1/issues/KYB-404", nil, nil)
		var p map[string]any
		_ = json.NewDecoder(res.Body).Decode(&p)
		res.Body.Close()
		if res.Header.Get("Content-Type") != "application/problem+json" {
			t.Fatalf("content type = %q", res.Header.Get("Content-Type"))
		}
		if p["status"] != float64(404) || p["code"] != "ISSUE_NOT_FOUND" || p["numeric_code"] != float64(6010) || p["instance"] != "/api/v1/issues/KYB-404" {
			t.Fatalf("problem = %v", p)
		}

		// Business rule rejections carry the specific reason in detail.
		code, body := c.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		expect(t, code, 409, body)
		if body["code"] != "ISSUE_TRANSITION_NOT_ALLOWED" || body["detail"] != "transition not allowed by workflow" {
			t.Fatalf("problem = %v", body)
		}

		// Titles are localized from Accept-Language (en, uz, ru).
		req, _ := http.NewRequest("GET", c.srv.URL+"/api/v1/issues/KYB-404", nil)
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept-Language", "uz")
		r2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var uz map[string]any
		_ = json.NewDecoder(r2.Body).Decode(&uz)
		r2.Body.Close()
		if uz["title"] != "Vazifa topilmadi." {
			t.Fatalf("uz title = %v", uz["title"])
		}

		// Unauthenticated requests are problems too.
		code, body = anon.do("GET", "/api/v1/me", nil)
		expect(t, code, 401, body)
		if body["code"] != "AUTH_REQUIRED" {
			t.Fatalf("problem = %v", body)
		}
	})
}

func issueKeys(t *testing.T, c *client, path string) string {
	t.Helper()
	code, body := c.do("GET", path, nil)
	expect(t, code, 200, body)
	var keys []string
	for _, it := range body["items"].([]any) {
		keys = append(keys, it.(map[string]any)["key"].(string))
	}
	return strings.Join(keys, ",")
}

func TestS18BacklogRanking(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		carol := anon.signedIn("carol@x.uz")
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "OPS", "name": "Ops"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "viewer"})
		for range 4 {
			alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		}
		alice.do("POST", "/api/v1/projects/OPS/issues", map[string]string{"title": "o", "type": "task"})

		if got := issueKeys(t, alice, "/api/v1/projects/KYB/issues"); got != "KYB-1,KYB-2,KYB-3,KYB-4" {
			t.Fatalf("AC1 new issues at the bottom: %s", got)
		}
		code, body := alice.do("POST", "/api/v1/issues/KYB-4/rank", map[string]string{"before": "KYB-1"})
		expect(t, code, 200, body)
		if body["rank"] == "" {
			t.Fatalf("rank missing: %v", body)
		}
		alice.do("POST", "/api/v1/issues/KYB-1/rank", map[string]string{"after": "KYB-3"})
		if got := issueKeys(t, alice, "/api/v1/projects/KYB/issues"); got != "KYB-4,KYB-2,KYB-3,KYB-1" {
			t.Fatalf("AC2/AC3 order: %s", got)
		}
		for _, bad := range []map[string]string{{}, {"after": "OPS-1"}, {"after": "KYB-99"}, {"after": "KYB-2", "before": "KYB-3"}} {
			code, body := alice.do("POST", "/api/v1/issues/KYB-1/rank", bad)
			expect(t, code, 422, body)
		}
		code, body = bob.do("POST", "/api/v1/issues/KYB-1/rank", map[string]string{"after": "KYB-2"})
		expect(t, code, 403, body)
		code, body = carol.do("POST", "/api/v1/issues/KYB-1/rank", map[string]string{"after": "KYB-2"})
		expect(t, code, 404, body)
	})
}

func TestS19Sprints(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		carol := anon.signedIn("carol@x.uz")
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "OPS", "name": "Ops"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "viewer"})
		for range 3 {
			alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		}

		code, s1 := alice.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "Sprint 1", "goal": "Ship the board"})
		expect(t, code, 201, s1)
		if s1["state"] != "planned" || s1["goal"] != "Ship the board" || s1["started_at"] != nil {
			t.Fatalf("AC1 created = %v", s1)
		}
		_, s2 := alice.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "Sprint 2"})
		_, opsSprint := alice.do("POST", "/api/v1/projects/OPS/sprints", map[string]string{"name": "Ops 1"})
		code, body := alice.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": " "})
		expect(t, code, 422, body)
		code, body = bob.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "x"})
		expect(t, code, 403, body)

		// AC2: plan issues into the sprint.
		move := func(key string, sprint any) (int, map[string]any) {
			_, is := alice.do("GET", "/api/v1/issues/"+key, nil)
			return alice.do("PATCH", "/api/v1/issues/"+key, map[string]any{"version": is["version"], "sprint_id": sprint})
		}
		for _, k := range []string{"KYB-1", "KYB-2", "KYB-3"} {
			code, body := move(k, s1["id"])
			expect(t, code, 200, body)
		}
		code, body = move("KYB-3", nil)
		expect(t, code, 200, body)
		if body["sprint_id"] != nil {
			t.Fatalf("back to backlog: %v", body)
		}
		code, body = move("KYB-3", opsSprint["id"])
		expect(t, code, 422, body)
		code, body = move("KYB-3", "not-a-uuid")
		expect(t, code, 422, body)
		if got := issueKeys(t, alice, "/api/v1/projects/KYB/issues?sprint="+s1["id"].(string)); got != "KYB-1,KYB-2" {
			t.Fatalf("sprint filter = %s", got)
		}
		if got := issueKeys(t, alice, "/api/v1/projects/KYB/issues?sprint=backlog"); got != "KYB-3" {
			t.Fatalf("backlog filter = %s", got)
		}

		// AC3: start; only one active sprint.
		code, body = alice.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/start", nil)
		expect(t, code, 200, body)
		if body["state"] != "active" || body["started_at"] == nil {
			t.Fatalf("started = %v", body)
		}
		code, body = alice.do("POST", "/api/v1/sprints/"+s2["id"].(string)+"/start", nil)
		expect(t, code, 409, body)
		if body["code"] != "SPRINT_ALREADY_ACTIVE" {
			t.Fatalf("problem = %v", body)
		}
		code, body = alice.do("POST", "/api/v1/sprints/"+s2["id"].(string)+"/complete", nil)
		expect(t, code, 409, body)
		code, body = carol.do("POST", "/api/v1/sprints/"+s2["id"].(string)+"/start", nil)
		expect(t, code, 404, body)
		code, body = alice.do("POST", "/api/v1/sprints/not-a-uuid/start", nil)
		expect(t, code, 404, body)

		code, body = alice.do("GET", "/api/v1/projects/KYB/sprints", nil)
		expect(t, code, 200, body)
		if items := body["items"].([]any); len(items) != 2 || items[0].(map[string]any)["name"] != "Sprint 1" {
			t.Fatalf("list = %v", items)
		}

		// AC4: complete returns unfinished issues to the backlog.
		alice.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "in_progress"})
		alice.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		code, body = bob.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/complete", nil)
		expect(t, code, 403, body)
		code, body = alice.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/complete", nil)
		expect(t, code, 200, body)
		if body["completed"] != float64(1) || body["returned"] != float64(1) || body["sprint"].(map[string]any)["state"] != "closed" {
			t.Fatalf("completion = %v", body)
		}
		if got := issueKeys(t, alice, "/api/v1/projects/KYB/issues?sprint=backlog"); got != "KYB-2,KYB-3" {
			t.Fatalf("backlog after completion = %s", got)
		}
		code, body = move("KYB-2", s1["id"])
		expect(t, code, 422, body) // closed sprints take no issues
		code, body = alice.do("POST", "/api/v1/sprints/"+s2["id"].(string)+"/start", nil)
		expect(t, code, 200, body) // the next sprint can start now
	})
}

// inbox polls until the user's notifications satisfy ok (the relay delivers asynchronously).
func inbox(t *testing.T, c *client, ok func(items []any, unread float64) bool) ([]any, float64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, body := c.do("GET", "/api/v1/notifications", nil)
		expect(t, code, 200, body)
		items, unread := body["items"].([]any), body["unread"].(float64)
		if ok(items, unread) {
			return items, unread
		}
		if time.Now().After(deadline) {
			t.Fatalf("notifications never matched; last = %v (unread %v)", items, unread)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestS21Reporter(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		_, me := alice.do("GET", "/api/v1/me", nil)
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		code, is := alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		expect(t, code, 201, is)
		if is["reporter_id"] != me["id"] {
			t.Fatalf("reporter_id = %v, want %v", is["reporter_id"], me["id"])
		}
	})
}

func TestS23Notifications(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		alice := anon.signedIn("alice@x.uz")
		bob := anon.signedIn("bob@x.uz")
		carol := anon.signedIn("carol@x.uz")
		eve := anon.signedIn("eve@x.uz") // not a member
		_, bobMe := bob.do("GET", "/api/v1/me", nil)
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "member"})
		alice.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "carol@x.uz", "role": "member"})
		_, is := alice.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login page", "type": "task"})

		// AC1: assigned.
		code, body := alice.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "assignee_id": bobMe["id"]})
		expect(t, code, 200, body)
		items, _ := inbox(t, bob, func(items []any, unread float64) bool { return len(items) == 1 && unread == 1 })
		n := items[0].(map[string]any)
		if n["kind"] != "assigned" || n["issue_key"] != "KYB-1" || n["issue_title"] != "Login page" || n["actor_name"] != "Tester" || n["read"] != false {
			t.Fatalf("assigned notification = %v", n)
		}

		// AC2 + AC3: Carol comments mentioning Bob and an outsider; Alice (reporter) is told, Bob is mentioned once.
		code, body = carol.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "@bob@x.uz @eve@x.uz can you check?"})
		expect(t, code, 201, body)
		items, _ = inbox(t, bob, func(items []any, _ float64) bool { return len(items) == 2 })
		if k := items[0].(map[string]any)["kind"]; k != "mentioned" {
			t.Fatalf("bob newest = %v (mention wins over commented)", k)
		}
		items, _ = inbox(t, alice, func(items []any, _ float64) bool { return len(items) == 1 })
		if k := items[0].(map[string]any)["kind"]; k != "commented" {
			t.Fatalf("reporter got %v", k)
		}
		// The author and the non-member get nothing (give the relay time to prove a negative).
		time.Sleep(300 * time.Millisecond)
		if _, body := carol.do("GET", "/api/v1/notifications", nil); len(body["items"].([]any)) != 0 {
			t.Fatalf("author notified: %v", body)
		}
		if _, body := eve.do("GET", "/api/v1/notifications", nil); len(body["items"].([]any)) != 0 {
			t.Fatalf("non-member notified: %v", body)
		}

		// AC4: read state; users only touch their own notifications.
		bobItems, _ := inbox(t, bob, func(items []any, _ float64) bool { return len(items) == 2 })
		first := bobItems[0].(map[string]any)["id"].(string)
		code, body = alice.do("POST", "/api/v1/notifications/"+first+"/read", nil)
		expect(t, code, 404, body)
		code, body = bob.do("POST", "/api/v1/notifications/"+first+"/read", nil)
		expect(t, code, 204, body)
		code, body = bob.do("GET", "/api/v1/notifications?unread=true", nil)
		expect(t, code, 200, body)
		if len(body["items"].([]any)) != 1 || body["unread"] != float64(1) {
			t.Fatalf("unread view = %v", body)
		}
		code, body = bob.do("POST", "/api/v1/notifications/read-all", nil)
		expect(t, code, 204, body)
		if _, body := bob.do("GET", "/api/v1/notifications", nil); body["unread"] != float64(0) {
			t.Fatalf("after read-all = %v", body)
		}
	})
}

func TestS25EstimatesAndSprintDates(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("est@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, is := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "story"})
		if is["estimate"] != nil {
			t.Fatalf("new issue estimate = %v", is["estimate"])
		}
		code, body := c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "estimate": 2.5})
		expect(t, code, 200, body)
		if body["estimate"] != 2.5 {
			t.Fatalf("estimate = %v", body["estimate"])
		}
		for _, bad := range []any{-1, 1000, 1.25} {
			code, b := c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": body["version"], "estimate": bad})
			expect(t, code, 422, b)
		}
		code, body = c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": body["version"], "estimate": nil})
		expect(t, code, 200, body)
		if body["estimate"] != nil {
			t.Fatalf("cleared estimate = %v", body["estimate"])
		}

		_, s1 := c.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S1"})
		code, body = c.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/start", map[string]string{"ends_at": "2001-01-01T00:00:00Z"})
		expect(t, code, 422, body)
		end := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Second)
		code, body = c.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/start", map[string]string{"ends_at": end.Format(time.RFC3339)})
		expect(t, code, 200, body)
		if got, _ := time.Parse(time.RFC3339, body["ends_at"].(string)); !got.Equal(end) {
			t.Fatalf("ends_at = %v, want %v", body["ends_at"], end)
		}
		_, s2 := c.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S2"})
		c.do("POST", "/api/v1/sprints/"+s1["id"].(string)+"/complete", nil)
		code, body = c.do("POST", "/api/v1/sprints/"+s2["id"].(string)+"/start", nil) // no body: default 14 days
		expect(t, code, 200, body)
		st, _ := time.Parse(time.RFC3339, body["started_at"].(string))
		en, _ := time.Parse(time.RFC3339, body["ends_at"].(string))
		if d := en.Sub(st); d < 14*24*time.Hour-time.Second || d > 14*24*time.Hour+time.Second {
			t.Fatalf("default length = %v", d)
		}
	})
}
