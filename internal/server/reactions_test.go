package server_test

import (
	"net/url"
	"testing"
)

// Comment reactions (F4): anyone who can see an issue toggles emoji on its comments.
func TestCommentReactions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		_, c := lead.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "ready"})
		if r, _ := c["reactions"].([]any); len(r) != 0 {
			t.Fatalf("new comment reactions = %v", c["reactions"])
		}
		base := "/api/v1/comments/" + c["id"].(string) + "/reactions/"
		thumbs := base + url.PathEscape("👍")
		for _, who := range []*client{dev, dev, lead} {
			code, b := who.do("PUT", thumbs, nil)
			expect(t, code, 204, b)
		}
		code, b := dev.do("PUT", base+url.PathEscape("🚀"), nil)
		expect(t, code, 204, b)
		code, b = dev.do("PUT", base+"smile", nil)
		expect(t, code, 422, b)
		code, b = eve.do("PUT", thumbs, nil)
		expect(t, code, 404, b)
		code, b = dev.do("PUT", "/api/v1/comments/00000000-0000-4000-8000-0000000000ff/reactions/"+url.PathEscape("👍"), nil)
		expect(t, code, 404, b)

		_, list := lead.do("GET", "/api/v1/issues/KYB-1/comments", nil)
		rs := list["items"].([]any)[0].(map[string]any)["reactions"].([]any)
		first := rs[0].(map[string]any)
		if len(rs) != 2 || first["emoji"] != "👍" || first["count"] != 2.0 || first["mine"] != true || rs[1].(map[string]any)["mine"] != false {
			t.Fatalf("reactions = %v", rs)
		}
		for range 2 {
			code, b = lead.do("DELETE", thumbs, nil)
			expect(t, code, 204, b)
		}
		_, list = lead.do("GET", "/api/v1/issues/KYB-1/comments", nil)
		rs = list["items"].([]any)[0].(map[string]any)["reactions"].([]any)
		if first := rs[0].(map[string]any); first["count"] != 1.0 || first["mine"] != false {
			t.Fatalf("after unreact = %v", rs)
		}
	})
}

// Presence (F4): who is viewing an issue right now, from heartbeats.
func TestIssuePresence(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		const url = "/api/v1/issues/KYB-1/presence"
		_, v := lead.do("GET", url, nil)
		if len(v["items"].([]any)) != 0 {
			t.Fatalf("nobody yet = %v", v)
		}
		for _, who := range []*client{lead, dev, lead} {
			code, b := who.do("POST", url, nil)
			expect(t, code, 204, b)
		}
		code, b := eve.do("POST", url, nil)
		expect(t, code, 404, b)
		code, b = eve.do("GET", url, nil)
		expect(t, code, 404, b)
		_, v = dev.do("GET", url, nil)
		items := v["items"].([]any)
		if len(items) != 2 || items[0].(map[string]any)["name"] == "" {
			t.Fatalf("viewers = %v", v)
		}
	})
}
