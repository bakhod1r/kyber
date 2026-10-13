package server_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

// hasAnnouncement reports whether the inbox holds an announcement titled title.
func hasAnnouncement(title, project string) func(map[string]any) bool {
	return func(b map[string]any) bool {
		for _, it := range b["items"].([]any) {
			n := it.(map[string]any)
			if n["kind"] == "announcement" && n["issue_title"] == title && n["project_key"] == project {
				return true
			}
		}
		return false
	}
}

// Announcements: admins tell a whole project or workspace; every member is notified.
func TestAnnouncements(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		boss := anon.signedIn("boss@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})

		const url = "/api/v1/projects/KYB/announcements"
		code, b := dev.do("POST", url, map[string]string{"title": "Mine"})
		expect(t, code, 403, b)
		code, b = lead.do("POST", url, map[string]string{"title": " "})
		expect(t, code, 422, b)
		code, a := lead.do("POST", url, map[string]string{"title": "Release Friday", "body": "Deploy at **16:00**"})
		expect(t, code, 201, a)
		if a["project_key"] != "KYB" || a["author_id"] == "" {
			t.Fatalf("announcement = %v", a)
		}
		code, list := dev.do("GET", url, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 1 {
			t.Fatalf("feed = %v", list)
		}
		code, b = out.do("GET", url, nil)
		expect(t, code, 404, b)
		eventuallyJSON(t, dev, "/api/v1/notifications", hasAnnouncement("Release Friday", "KYB"))

		// Workspace-wide: only configured administrators in single-tenant mode.
		code, b = lead.do("POST", "/api/v1/announcements", map[string]string{"title": "Holiday"})
		expect(t, code, 403, b)
		code, a = boss.do("POST", "/api/v1/announcements", map[string]string{"title": "Holiday", "body": "Office closed Monday"})
		expect(t, code, 201, a)
		if a["project_key"] != nil {
			t.Fatalf("workspace announcement = %v", a)
		}
		code, list = out.do("GET", "/api/v1/announcements", nil)
		expect(t, code, 200, list)
		eventuallyJSON(t, dev, "/api/v1/notifications", hasAnnouncement("Holiday", ""))
		eventuallyJSON(t, lead, "/api/v1/notifications", hasAnnouncement("Holiday", ""))
	}, server.WithSiteAdmins(" Boss@x.uz , "))
}
