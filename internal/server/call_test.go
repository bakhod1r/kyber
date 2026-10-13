package server_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
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

// S56: signaling reaches only its recipient; ICE servers come with short-lived TURN credentials.
func TestCallSignaling(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "dev"})
		url := "/api/v1/conversations/" + ch["id"].(string)
		dev.do("POST", url+"/join", nil)
		qa.do("POST", url+"/join", nil)
		_, call := lead.do("POST", url+"/call", nil)
		dev.do("POST", url+"/call", nil)
		qa.do("POST", url+"/call", nil)
		devStream, qaStream := dev.stream(""), qa.stream("")

		sig := "/api/v1/calls/" + call["id"].(string) + "/signal"
		code, b := lead.do("POST", sig, map[string]string{"to": devMe["id"].(string), "kind": "offer", "data": `{"type":"offer","sdp":"v=0"}`})
		expect(t, code, 204, b)
		e := next(t, devStream, "call.signal")
		if !strings.Contains(e.data, `"kind":"offer"`) || !strings.Contains(e.data, `"call_id":"`+call["id"].(string)) || !strings.Contains(e.data, `v=0`) {
			t.Fatalf("signal = %+v", e)
		}
		quiet(t, qaStream)
		code, b = lead.do("POST", sig, map[string]string{"to": devMe["id"].(string), "kind": "bogus", "data": "x"})
		expect(t, code, 422, b)

		code, ice := lead.do("GET", "/api/v1/calls/ice", nil)
		expect(t, code, 200, ice)
		servers := ice["ice_servers"].([]any)
		if len(servers) != 2 || servers[1].(map[string]any)["username"] == "" || servers[1].(map[string]any)["credential"] == "" {
			t.Fatalf("ice = %v", ice)
		}
	}, server.WithICE([]string{"stun:stun.example.com:3478", "turn:turn.example.com:3478"}, "s3cret"))
}

// S60: present a doc or whiteboard to everyone in the call; the presenter steers where everyone looks.
func TestPresentToAll(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "dev"})
		url := "/api/v1/conversations/" + ch["id"].(string)
		dev.do("POST", url+"/join", nil)
		_, call := lead.do("POST", url+"/call", nil)
		dev.do("POST", url+"/call", nil)
		callURL := "/api/v1/calls/" + call["id"].(string)
		devStream := dev.stream("")

		code, b := lead.do("PUT", callURL+"/presentation", map[string]string{"kind": "whiteboard", "ref": "wb-1"})
		expect(t, code, 200, b)
		if p := b["presentation"].(map[string]any); p["kind"] != "whiteboard" || p["ref"] != "wb-1" {
			t.Fatalf("presentation = %v", b)
		}
		if e := next(t, devStream, "call.updated"); !strings.Contains(e.data, `"ref":"wb-1"`) {
			t.Fatalf("call.updated = %+v", e)
		}
		code, b = lead.do("PUT", callURL+"/presentation", map[string]string{"kind": "slides", "ref": "x"})
		expect(t, code, 422, b)

		code, b = lead.do("POST", callURL+"/presentation/follow", map[string]any{"x": 0.25, "y": 0.5, "zoom": 1})
		expect(t, code, 204, b)
		if e := next(t, devStream, "call.follow"); !strings.Contains(e.data, `"y":0.5`) {
			t.Fatalf("follow = %+v", e)
		}
		code, b = dev.do("POST", callURL+"/presentation/follow", map[string]any{"x": 0, "y": 0, "zoom": 1})
		expect(t, code, 403, b)
		code, b = dev.do("DELETE", callURL+"/presentation", nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", callURL+"/presentation", nil)
		expect(t, code, 200, b)
		if b["presentation"] != nil {
			t.Fatalf("stopped = %v", b)
		}
	})
}
