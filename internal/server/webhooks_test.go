package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

const githubSecret = "gh-secret"

// receiver records the webhook deliveries it gets.
type receiver struct {
	mu   sync.Mutex
	reqs []*http.Request
	body [][]byte
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	rc.reqs, rc.body = append(rc.reqs, r), append(rc.body, b)
	rc.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (rc *receiver) wait(t *testing.T, n int) (*http.Request, []byte) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rc.mu.Lock()
		if len(rc.reqs) >= n {
			defer rc.mu.Unlock()
			return rc.reqs[n-1], rc.body[n-1]
		}
		rc.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("webhook delivery %d never arrived", n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// github posts a GitHub webhook delivery, signed with secret unless it is "".
func (c *client) github(event, secret string, payload any) (int, map[string]any) {
	c.t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/v1/integrations/github", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", domain.Sign(secret, b))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	spec.check(c.t, req, res)
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// F5: project admins manage signed outgoing webhooks; GitHub pushes and pull requests become
// development links on the issues they mention.
func TestWebhooks(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		rc := &receiver{}
		target := httptest.NewServer(rc)
		t.Cleanup(target.Close)
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		const hooks = "/api/v1/projects/KYB/webhooks"

		code, w := lead.do("POST", hooks, map[string]any{"url": target.URL + "/hook", "events": []string{"issue.created"}})
		expect(t, code, 201, w)
		secret, _ := w["secret"].(string)
		if len(secret) != 64 || w["active"] != true {
			t.Fatalf("created = %v", w)
		}
		id := w["id"].(string)
		code, b := lead.do("POST", hooks, map[string]any{"url": "http://example.com/hook", "events": []string{"*"}})
		expect(t, code, 422, b)
		code, b = vic.do("POST", hooks, map[string]any{"url": "https://example.com/hook", "events": []string{"*"}})
		expect(t, code, 403, b)
		code, b = out.do("POST", hooks, map[string]any{"url": "https://example.com/hook", "events": []string{"*"}})
		expect(t, code, 404, b)
		code, list := lead.do("GET", hooks, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["secret"] != nil {
			t.Fatalf("list = %v", list)
		}
		code, b = vic.do("GET", hooks, nil)
		expect(t, code, 403, b)
		code, b = out.do("GET", hooks, nil)
		expect(t, code, 404, b)

		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret plan", "type": "bug"})
		req, body := rc.wait(t, 1)
		if req.Header.Get("X-Kyber-Event") != "issue.created" || req.Header.Get("X-Kyber-Signature") != domain.Sign(secret, body) ||
			req.Header.Get("X-Kyber-Delivery") == "" || strings.Contains(string(body), "Secret plan") {
			t.Fatalf("delivery %v %s", req.Header, body)
		}
		var p map[string]any
		_ = json.Unmarshal(body, &p)
		if p["event"] != "issue.created" || p["project"] != "KYB" || p["data"].(map[string]any)["key"] != "KYB-1" || p["id"] != req.Header.Get("X-Kyber-Delivery") {
			t.Fatalf("payload = %s", body)
		}
		deliveries := eventuallyJSON(t, lead, hooks+"/"+id+"/deliveries", func(m map[string]any) bool {
			items := m["items"].([]any)
			return len(items) == 1 && items[0].(map[string]any)["status"] == "delivered"
		})
		if d := deliveries["items"].([]any)[0].(map[string]any); d["last_status"] != float64(204) || d["attempts"] != float64(1) {
			t.Fatalf("delivery = %v", d)
		}
		code, b = vic.do("GET", hooks+"/"+id+"/deliveries", nil)
		expect(t, code, 403, b)

		code, b = lead.do("PUT", hooks+"/"+id, map[string]any{"url": target.URL + "/v2", "events": []string{"*"}, "active": false})
		expect(t, code, 200, b)
		if b["active"] != false || b["secret"] != nil || b["url"] != target.URL+"/v2" {
			t.Fatalf("updated = %v", b)
		}
		code, b = lead.do("PUT", hooks+"/"+id, map[string]any{"url": "ftp://x", "events": []string{"*"}})
		expect(t, code, 422, b)
		code, b = vic.do("PUT", hooks+"/"+id, map[string]any{"url": "https://x.io", "events": []string{"*"}})
		expect(t, code, 403, b)
		code, b = lead.do("PUT", hooks+"/nope", map[string]any{"url": "https://x.io", "events": []string{"*"}})
		expect(t, code, 404, b)

		// GitHub: signed pushes and pull requests link commits and PRs to the issues they mention.
		code, b = anon.github("push", "", map[string]any{})
		expect(t, code, 401, b)
		code, b = anon.github("push", "wrong", map[string]any{})
		expect(t, code, 401, b)
		code, b = anon.github("ping", githubSecret, map[string]any{"zen": "hi"})
		expect(t, code, 200, b)
		code, b = anon.github("push", githubSecret, map[string]any{
			"repository": map[string]any{"full_name": "acme/app"},
			"commits": []any{
				map[string]any{"id": "abcdef1234567", "url": "https://github.com/acme/app/commit/abcdef1234567", "message": "KYB-1 fix the crash\n\ndetails"},
				map[string]any{"id": "0000000", "url": "https://github.com/acme/app/commit/0000000", "message": "chore: no key"},
			},
		})
		expect(t, code, 200, b)
		if b["linked"] != float64(1) {
			t.Fatalf("push linked %v", b)
		}
		code, b = anon.github("pull_request", githubSecret, map[string]any{
			"repository": map[string]any{"full_name": "acme/app"},
			"pull_request": map[string]any{"number": 7, "html_url": "https://github.com/acme/app/pull/7", "title": "Crash fix",
				"state": "closed", "merged": true, "head": map[string]any{"ref": "fix/KYB-1-crash"}},
		})
		expect(t, code, 200, b)
		code, b = anon.github("push", githubSecret, "not an object")
		expect(t, code, 400, b)

		code, dev := vic.do("GET", "/api/v1/issues/KYB-1/development", nil)
		expect(t, code, 200, dev)
		items := dev["items"].([]any)
		if len(items) != 2 {
			t.Fatalf("development = %v", dev)
		}
		kinds := map[string]map[string]any{}
		for _, it := range items {
			m := it.(map[string]any)
			kinds[m["kind"].(string)] = m
		}
		if kinds["pull_request"]["state"] != "merged" || kinds["pull_request"]["ref"] != "7" || kinds["commit"]["title"] != "KYB-1 fix the crash" {
			t.Fatalf("development = %v", dev)
		}
		code, b = out.do("GET", "/api/v1/issues/KYB-1/development", nil)
		expect(t, code, 404, b)

		code, b = vic.do("DELETE", hooks+"/"+id, nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", hooks+"/"+id, nil)
		expect(t, code, 204, b)
		code, b = lead.do("DELETE", hooks+"/"+id, nil)
		expect(t, code, 404, b)
		code, b = lead.do("GET", hooks+"/"+id+"/deliveries", nil)
		expect(t, code, 404, b)
	}, server.WithWebhooks(true, 20*time.Millisecond), server.WithGitHubSecret(githubSecret))
}

// Without a configured secret the GitHub endpoint does not exist.
func TestGitHubDisabled(t *testing.T) {
	srv := httptest.NewServer(server.NewInMemory(slog.New(slog.NewTextHandler(io.Discard, nil)), server.WithContext(t.Context())))
	t.Cleanup(srv.Close)
	code, b := (&client{t: t, srv: srv}).github("push", "any", map[string]any{})
	expect(t, code, 404, b)
}
