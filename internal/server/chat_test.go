package server_test

import (
	"testing"
)

// S50 Chat core: channels, DMs, messages, unread; private conversations look missing to outsiders.
func TestChat(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, leadMe := lead.do("GET", "/api/v1/me", nil)

		code, general := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general", "topic": "All hands"})
		expect(t, code, 201, general)
		if general["kind"] != "channel" || general["member"] != true || general["private"] != false {
			t.Fatalf("channel = %v", general)
		}
		code, b := dev.do("POST", "/api/v1/channels", map[string]any{"name": "General"})
		expect(t, code, 409, b)
		code, b = dev.do("POST", "/api/v1/channels", map[string]any{"name": "no spaces"})
		expect(t, code, 422, b)
		_, secret := lead.do("POST", "/api/v1/channels", map[string]any{"name": "secret", "private": true})
		gURL, sURL := "/api/v1/conversations/"+general["id"].(string), "/api/v1/conversations/"+secret["id"].(string)

		_, list := dev.do("GET", "/api/v1/conversations", nil)
		if items := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["member"] != false {
			t.Fatalf("dev list = %v", list)
		}
		for _, r := range []struct{ method, path string }{{"GET", sURL}, {"GET", sURL + "/messages"}, {"POST", sURL + "/join"}} {
			code, b := dev.do(r.method, r.path, nil)
			expect(t, code, 404, b)
		}
		code, b = dev.do("POST", gURL+"/messages", map[string]string{"body": "hi"})
		expect(t, code, 403, b)
		code, b = dev.do("POST", gURL+"/join", nil)
		expect(t, code, 204, b)

		for _, text := range []string{"one", "two", "three"} {
			code, b = lead.do("POST", gURL+"/messages", map[string]string{"body": text})
			expect(t, code, 201, b)
		}
		code, b = lead.do("POST", gURL+"/messages", map[string]string{"body": " "})
		expect(t, code, 422, b)
		_, g := dev.do("GET", gURL, nil)
		if g["unread"] != float64(3) {
			t.Fatalf("unread = %v", g)
		}
		_, page := dev.do("GET", gURL+"/messages?limit=2", nil)
		items := page["items"].([]any)
		if len(items) != 2 || items[0].(map[string]any)["body"] != "three" || items[0].(map[string]any)["author_id"] != leadMe["id"] {
			t.Fatalf("page = %v", page)
		}
		last := items[1].(map[string]any)["id"].(string)
		_, page = dev.do("GET", gURL+"/messages?before="+last, nil)
		if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["body"] != "one" {
			t.Fatalf("older = %v", page)
		}
		code, b = dev.do("POST", gURL+"/read", nil)
		expect(t, code, 204, b)
		if _, g := dev.do("GET", gURL, nil); g["unread"] != float64(0) {
			t.Fatalf("after read = %v", g)
		}

		msgURL := "/api/v1/messages/" + last
		code, b = dev.do("PATCH", msgURL, map[string]string{"body": "hacked"})
		expect(t, code, 403, b)
		code, b = lead.do("PATCH", msgURL, map[string]string{"body": "two (edited)"})
		expect(t, code, 200, b)
		if b["body"] != "two (edited)" || b["edited_at"] == nil {
			t.Fatalf("edited = %v", b)
		}
		code, b = lead.do("DELETE", msgURL, nil)
		expect(t, code, 204, b)
		_, page = dev.do("GET", gURL+"/messages", nil)
		if m := page["items"].([]any)[1].(map[string]any); m["deleted"] != true || m["body"] != "" {
			t.Fatalf("deleted message = %v", m)
		}

		// Private channel: invite, then leave.
		code, b = lead.do("POST", sURL+"/members", map[string]string{"user_id": devMe["id"].(string)})
		expect(t, code, 204, b)
		code, b = dev.do("GET", sURL, nil)
		expect(t, code, 200, b)
		code, b = dev.do("POST", sURL+"/leave", nil)
		expect(t, code, 204, b)
		code, b = dev.do("GET", sURL, nil)
		expect(t, code, 404, b)

		// Direct messages: the same people get the same DM; a third person cannot see it.
		code, dm := dev.do("POST", "/api/v1/dms", map[string]any{"user_ids": []string{leadMe["id"].(string)}})
		expect(t, code, 200, dm)
		_, again := lead.do("POST", "/api/v1/dms", map[string]any{"user_ids": []string{devMe["id"].(string)}})
		if dm["kind"] != "dm" || again["id"] != dm["id"] || len(dm["member_ids"].([]any)) != 2 {
			t.Fatalf("dm = %v, again = %v", dm, again)
		}
		code, b = qa.do("GET", "/api/v1/conversations/"+dm["id"].(string), nil)
		expect(t, code, 404, b)
		code, b = dev.do("POST", "/api/v1/dms", map[string]any{"user_ids": []string{"00000000-0000-4000-8000-00000000dead"}})
		expect(t, code, 422, b)
	})
}

// S52: threads and reactions.
func TestChatThreadsAndReactions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		url := "/api/v1/conversations/" + ch["id"].(string)
		dev.do("POST", url+"/join", nil)
		_, parent := lead.do("POST", url+"/messages", map[string]string{"body": "release today?"})
		pURL := "/api/v1/messages/" + parent["id"].(string)

		code, reply := dev.do("POST", pURL+"/replies", map[string]string{"body": "after lunch"})
		expect(t, code, 201, reply)
		if reply["thread_id"] != parent["id"] {
			t.Fatalf("reply = %v", reply)
		}
		code, b := dev.do("POST", "/api/v1/messages/"+reply["id"].(string)+"/replies", map[string]string{"body": "nested"})
		expect(t, code, 422, b)
		code, b = qa.do("POST", pURL+"/replies", map[string]string{"body": "not joined"})
		expect(t, code, 403, b)
		_, thread := qa.do("GET", pURL+"/replies", nil)
		if items := thread["items"].([]any); len(items) != 1 || items[0].(map[string]any)["body"] != "after lunch" {
			t.Fatalf("thread = %v", thread)
		}
		_, page := dev.do("GET", url+"/messages", nil)
		if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["reply_count"] != float64(1) {
			t.Fatalf("history = %v", page)
		}

		code, m := dev.do("POST", pURL+"/reactions", map[string]string{"emoji": "🎉"})
		expect(t, code, 200, m)
		rs := m["reactions"].([]any)
		if len(rs) != 1 || rs[0].(map[string]any)["emoji"] != "🎉" || rs[0].(map[string]any)["user_ids"].([]any)[0] != devMe["id"] {
			t.Fatalf("reactions = %v", m)
		}
		code, b = dev.do("POST", pURL+"/reactions", map[string]string{"emoji": "two words"})
		expect(t, code, 422, b)
		_, m = dev.do("POST", pURL+"/reactions", map[string]string{"emoji": "🎉"})
		if len(m["reactions"].([]any)) != 0 {
			t.Fatalf("toggle off = %v", m)
		}
	})
}
