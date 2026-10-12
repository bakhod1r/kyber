package server_test

import (
	"fmt"
	"testing"
)

// ADR-0003 authorization matrix: every issue endpoint × role → status, on all backends.
// Outsiders always get 404 (no existence leak); members of the project lacking the permission 403.
func TestS46AccessMatrix(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		admin := anon.signedIn("admin@x.uz")
		member := anon.signedIn("member@x.uz")
		viewer := anon.signedIn("viewer@x.uz")
		outsider := anon.signedIn("outsider@x.uz")
		admin.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		admin.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "member@x.uz", "role": "member"})
		admin.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "viewer@x.uz", "role": "viewer"})

		roles := []struct {
			name string
			c    *client
		}{{"admin", admin}, {"member", member}, {"viewer", viewer}, {"outsider", outsider}}
		// Each row: request builder (fresh issue per call where it mutates) and expected status per role.
		rows := []struct {
			name string
			do   func(c *client, n int) (int, map[string]any)
			want [4]int
		}{
			{"browse issue", func(c *client, n int) (int, map[string]any) { return c.do("GET", issue(n), nil) }, [4]int{200, 200, 200, 404}},
			{"list issues", func(c *client, n int) (int, map[string]any) { return c.do("GET", "/api/v1/projects/KYB/issues", nil) }, [4]int{200, 200, 200, 404}},
			{"create issue", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "t", "type": "task"})
			}, [4]int{201, 201, 403, 404}},
			{"edit others' issue", func(c *client, n int) (int, map[string]any) {
				return c.do("PATCH", issue(n), map[string]any{"version": 1, "title": "edited"})
			}, [4]int{200, 200, 403, 404}},
			{"assign", func(c *client, n int) (int, map[string]any) {
				return c.do("PATCH", issue(n), map[string]any{"version": 1, "assignee_id": nil})
			}, [4]int{200, 200, 403, 404}},
			{"transition", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", issue(n)+"/transitions", map[string]string{"to": "in_progress"})
			}, [4]int{200, 200, 403, 404}},
			{"close (to Done)", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", issue(n)+"/transitions", map[string]string{"to": "done"})
			}, [4]int{200, 200, 403, 404}},
			{"comment", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", issue(n)+"/comments", map[string]string{"body": "hi"})
			}, [4]int{201, 201, 403, 404}},
			{"read comments", func(c *client, n int) (int, map[string]any) { return c.do("GET", issue(n)+"/comments", nil) }, [4]int{200, 200, 200, 404}},
			{"read history", func(c *client, n int) (int, map[string]any) { return c.do("GET", issue(n)+"/history", nil) }, [4]int{200, 200, 200, 404}},
			{"rank", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", issue(n)+"/rank", map[string]string{"before": "KYB-1"})
			}, [4]int{200, 200, 403, 404}},
			{"link", func(c *client, n int) (int, map[string]any) {
				return c.do("POST", issue(n)+"/links", map[string]string{"type": "relates", "to_key": "KYB-1"})
			}, [4]int{201, 201, 403, 404}},
			{"read workflow", func(c *client, n int) (int, map[string]any) { return c.do("GET", "/api/v1/projects/KYB/workflow", nil) }, [4]int{200, 200, 200, 404}},
			{"change workflow", func(c *client, n int) (int, map[string]any) {
				return c.do("PUT", "/api/v1/projects/KYB/workflow", map[string]any{"initial": "todo", "statuses": []map[string]string{
					{"id": "todo", "name": "To Do", "category": "todo"}, {"id": "in_progress", "name": "In Progress", "category": "in_progress"},
					{"id": "done", "name": "Done", "category": "done"}}})
			}, [4]int{200, 403, 403, 404}},
		}
		admin.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "anchor", "type": "task"}) // KYB-1
		n := 1
		for _, row := range rows {
			for i, r := range roles {
				n++
				code, b := admin.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "target", "type": "task"})
				expect(t, code, 201, b)
				if got, body := row.do(r.c, n); got != row.want[i] {
					t.Errorf("%s as %s: status %d, want %d (%v)", row.name, r.name, got, row.want[i], body)
				}
				if row.name == "create issue" && row.want[i] == 201 {
					n++ // the role created one more issue
				}
			}
		}

		// Attribute rule: a reporter keeps "edit own issue" after being demoted to viewer.
		code, own := member.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "mine", "type": "task"})
		expect(t, code, 201, own)
		admin.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "member@x.uz", "role": "viewer"})
		code, b := member.do("PATCH", "/api/v1/issues/"+own["key"].(string), map[string]any{"version": own["version"], "title": "still mine"})
		expect(t, code, 200, b)
		code, b = member.do("POST", "/api/v1/issues/"+own["key"].(string)+"/transitions", map[string]string{"to": "done"})
		expect(t, code, 403, b)
	})
}

func issue(n int) string { return fmt.Sprintf("/api/v1/issues/KYB-%d", n) }
