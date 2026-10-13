package server_test

import (
	"testing"
	"time"
)

// Watchers: the reporter, assignee and commenters watch an issue; anyone who can see it may
// watch; watchers hear about comments and status changes.
func TestWatchers(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		eve := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		for _, e := range []string{"dev@x.uz", "qa@x.uz"} {
			lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": e, "role": "member"})
		}
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		const url = "/api/v1/issues/KYB-1/watchers"
		_, w := lead.do("GET", url, nil)
		if w["watching"] != true || len(w["items"].([]any)) != 1 {
			t.Fatalf("reporter watches = %v", w)
		}
		code, b := qa.do("POST", url, nil)
		expect(t, code, 204, b)
		code, b = eve.do("POST", url, nil)
		expect(t, code, 404, b)
		_, w = qa.do("GET", url, nil)
		if w["watching"] != true || len(w["items"].([]any)) != 2 {
			t.Fatalf("after watch = %v", w)
		}

		dev.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "starting"})
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		eventuallyJSON(t, qa, "/api/v1/notifications", func(b map[string]any) bool { return len(b["items"].([]any)) == 2 })
		_, inbox := qa.do("GET", "/api/v1/notifications", nil)
		kinds := map[string]bool{}
		for _, n := range inbox["items"].([]any) {
			kinds[n.(map[string]any)["kind"].(string)] = true
		}
		if !kinds["commented"] || !kinds["transitioned"] {
			t.Fatalf("qa inbox = %v", inbox)
		}
		// The commenter now watches; nobody hears about their own change.
		_, w = dev.do("GET", url, nil)
		if w["watching"] != true {
			t.Fatalf("commenter watches = %v", w)
		}
		code, b = qa.do("DELETE", url, nil)
		expect(t, code, 204, b)
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "todo"})
		time.Sleep(300 * time.Millisecond)
		if _, inbox := qa.do("GET", "/api/v1/notifications", nil); len(inbox["items"].([]any)) != 2 {
			t.Fatalf("unwatched qa still notified: %v", inbox)
		}
		_, own := dev.do("GET", "/api/v1/notifications", nil)
		for _, n := range own["items"].([]any) {
			if n.(map[string]any)["kind"] == "transitioned" {
				t.Fatalf("dev told about own change: %v", own)
			}
		}
	})
}
