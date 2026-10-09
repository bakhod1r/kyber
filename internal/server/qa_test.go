package server_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
)

// QA-2: starting a sprint right after planning must count the planned scope.
func TestQA2BurndownScopeWhenStartingImmediately(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("lead@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, is := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
		_, sp := c.do("POST", "/api/v1/projects/KYB/sprints", map[string]string{"name": "S1"})
		sid := sp["id"].(string)
		c.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "sprint_id": sid, "estimate": 5})
		c.do("POST", "/api/v1/sprints/"+sid+"/start", nil) // no pause: the relay has not run yet
		eventuallyJSON(t, c, "/api/v1/sprints/"+sid+"/burndown", func(b map[string]any) bool {
			return b["start_points"] == float64(5) && b["start_issues"] == float64(1)
		})
	})
}

// QA-4: Jira summaries are one line of at most 255 characters; names are bounded too.
func TestQA4Limits(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("lead@x.uz")
		c.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		for _, title := range []string{strings.Repeat("x", 256), "two\nlines"} {
			code, b := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": title, "type": "task"})
			expect(t, code, 422, b)
		}
		code, b := c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": strings.Repeat("é", 255), "type": "task"})
		expect(t, code, 201, b)
		code, b = anon.do("POST", "/api/v1/auth/signup", map[string]string{"email": "n@x.uz", "name": strings.Repeat("n", 101), "password": "long enough pw"})
		expect(t, code, 422, b)
	})
}

// QA-5: outsiders cannot learn that a meeting exists; attendees get 403, others 404.
func TestQA5MeetingExistence(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		out := anon.signedIn("out@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		now := time.Now().UTC()
		_, m := lead.do("POST", "/api/v1/meetings", map[string]any{"title": "Secret", "starts_at": now.Add(time.Hour), "ends_at": now.Add(2 * time.Hour),
			"attendee_ids": []any{devMe["id"]}})
		code, b := out.do("DELETE", "/api/v1/meetings/"+m["id"].(string), nil)
		expect(t, code, 404, b)
		code, b = dev.do("DELETE", "/api/v1/meetings/"+m["id"].(string), nil)
		expect(t, code, 403, b)
	})
}

// QA-6: every API error is problem+json, including unknown routes and wrong methods.
func TestQA6ProblemJSONEverywhere(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		c := anon.signedIn("lead@x.uz")
		for _, r := range []struct {
			method, path string
			status       int
		}{{"GET", "/api/v1/nope", 404}, {"PUT", "/api/v1/projects", 405}, {"GET", "/api/v1/auth/google/start", 404}} {
			// Sent directly: deliberately undocumented calls must not feed the OpenAPI drift check.
			req, _ := http.NewRequest(r.method, c.srv.URL+r.path, nil)
			req.Header.Set("Authorization", "Bearer "+c.token)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != r.status || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/problem+json") {
				t.Errorf("%s %s = %d %q", r.method, r.path, res.StatusCode, res.Header.Get("Content-Type"))
			}
		}
	})
}

// QA-7: a workspace's meetings are hidden from non-members like every other endpoint.
func TestQA7MeetingsNeedMembership(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		apex := on(anon, "kyber.test")
		owner := apex.signedIn("owner@x.uz")
		stranger := apex.signedIn("stranger@x.uz")
		owner.do("POST", "/api/v1/workspaces", map[string]string{"slug": "zeta", "name": "Zeta"})
		code, b := on(stranger, "zeta.kyber.test").do("GET", "/api/v1/meetings", nil)
		expect(t, code, 404, b)
		code, b = on(stranger, "zeta.kyber.test").do("GET", "/api/v1/me", nil)
		expect(t, code, 200, b) // /me itself stays available everywhere
	}, server.WithBaseDomain("kyber.test"))
}

// QA parity: Jira permissions and data hygiene.
func TestQAParity(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		_, vicMe := vic.do("GET", "/api/v1/me", nil)
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})

		// Assignable User: a viewer cannot be the assignee.
		code, b := lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "assignee_id": vicMe["id"]})
		expect(t, code, 422, b)
		// Work On Issues: a viewer cannot log focus time, but can read the log.
		code, b = vic.do("POST", "/api/v1/focus", map[string]any{"issue_key": "KYB-1"})
		expect(t, code, 403, b)
		code, b = vic.do("GET", "/api/v1/issues/KYB-1/focus", nil)
		expect(t, code, 200, b)
		// Simplified workflow: To Do → Done directly.
		code, b = lead.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		expect(t, code, 200, b)
		// Meetings cannot be booked in the past.
		past := time.Now().UTC().Add(-2 * time.Hour)
		code, b = lead.do("POST", "/api/v1/meetings", map[string]any{"title": "Old", "starts_at": past, "ends_at": past.Add(time.Hour)})
		expect(t, code, 422, b)
		// Re-importing the project's own export does not duplicate its issues.
		res := lead.raw("GET", "/api/v1/projects/KYB/export.csv", nil, nil)
		csv, _ := io.ReadAll(res.Body)
		res.Body.Close()
		code, run := lead.do("POST", "/api/v1/projects/KYB/import/jira?dry_run=false", map[string]string{"csv": string(csv)})
		expect(t, code, 201, run)
		if len(run["items"].([]any)) != 0 || len(run["skipped"].([]any)) != 1 {
			t.Fatalf("self re-import = %v", run)
		}
	})
}
