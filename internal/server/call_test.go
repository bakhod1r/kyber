package server_test

import (
	"fmt"
	"strings"
	"testing"
)

// S55: huddles in a conversation — start/join, media state, leave; only members; at most 6 people.
func TestHuddle(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		eve := anon.signedIn("eve@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "dev", "private": true})
		lead.do("POST", "/api/v1/conversations/"+ch["id"].(string)+"/members", map[string]string{"user_id": devMe["id"].(string)})
		url := "/api/v1/conversations/" + ch["id"].(string) + "/call"

		code, none := dev.do("GET", url, nil)
		expect(t, code, 200, none)
		if none["call"] != nil {
			t.Fatalf("no call yet = %v", none)
		}
		devStream := dev.stream("")
		code, call := lead.do("POST", url, nil)
		expect(t, code, 200, call)
		if e := next(t, devStream, "call.updated"); !strings.Contains(e.data, call["id"].(string)) {
			t.Fatalf("call event = %+v", e)
		}
		code, call = dev.do("POST", url, nil)
		expect(t, code, 200, call)
		if ps := call["participants"].([]any); len(ps) != 2 || ps[1].(map[string]any)["audio"] != true || call["live"] != true {
			t.Fatalf("joined = %v", call)
		}
		code, b := eve.do("POST", url, nil)
		expect(t, code, 404, b)

		callURL := "/api/v1/calls/" + call["id"].(string)
		code, b = dev.do("PUT", callURL+"/media", map[string]bool{"audio": false, "video": true, "screen": true})
		expect(t, code, 200, b)
		if p := b["participants"].([]any)[1].(map[string]any); p["video"] != true || p["screen"] != true || p["audio"] != false {
			t.Fatalf("media = %v", b)
		}
		code, b = eve.do("PUT", callURL+"/media", map[string]bool{"video": true})
		expect(t, code, 404, b)
		code, b = eve.do("POST", callURL+"/leave", nil)
		expect(t, code, 404, b)

		for _, c := range []*client{lead, dev} {
			code, b = c.do("POST", callURL+"/leave", nil)
			expect(t, code, 204, b)
		}
		if _, now := dev.do("GET", url, nil); now["call"] != nil {
			t.Fatalf("after leaving = %v", now)
		}
		code, b = dev.do("POST", callURL+"/leave", nil)
		expect(t, code, 409, b)

		// A public channel caps the call at six people.
		_, pub := lead.do("POST", "/api/v1/channels", map[string]any{"name": "all"})
		pURL := "/api/v1/conversations/" + pub["id"].(string)
		for i := range 6 {
			c := anon.signedIn(fmt.Sprintf("p%d@x.uz", i))
			c.do("POST", pURL+"/join", nil)
			code, b = c.do("POST", pURL+"/call", nil)
			expect(t, code, 200, b)
		}
		code, b = lead.do("POST", pURL+"/call", nil)
		expect(t, code, 409, b)
		if b["code"] != "CALL_FULL" {
			t.Fatalf("full = %v", b)
		}
	})
}
