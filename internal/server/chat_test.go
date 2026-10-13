package server_test

import (
	"strings"
	"testing"
	"time"
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

// S52b: @email mentions in chat notify people who can read the conversation.
func TestChatMentions(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, pub := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		_, priv := lead.do("POST", "/api/v1/channels", map[string]any{"name": "secret", "private": true})
		lead.do("POST", "/api/v1/conversations/"+pub["id"].(string)+"/messages",
			map[string]string{"body": "@dev@x.uz @qa@x.uz @lead@x.uz please review"})
		lead.do("POST", "/api/v1/conversations/"+priv["id"].(string)+"/messages", map[string]string{"body": "@qa@x.uz psst"})

		eventuallyJSON(t, dev, "/api/v1/notifications", func(b map[string]any) bool { return len(b["items"].([]any)) == 1 })
		eventuallyJSON(t, qa, "/api/v1/notifications", func(b map[string]any) bool { return len(b["items"].([]any)) == 1 })
		time.Sleep(300 * time.Millisecond)
		_, inbox := qa.do("GET", "/api/v1/notifications", nil)
		items := inbox["items"].([]any)
		n := items[0].(map[string]any)
		if len(items) != 1 || n["kind"] != "mentioned" || n["conversation_id"] != pub["id"] || n["conversation_name"] != "general" ||
			!strings.Contains(n["excerpt"].(string), "please review") {
			t.Fatalf("qa inbox = %v (only the public channel: qa cannot read #secret)", inbox)
		}
		if _, own := lead.do("GET", "/api/v1/notifications", nil); len(own["items"].([]any)) != 0 {
			t.Fatalf("author told about own mention: %v", own)
		}
	})
}

// S52c: issue keys in a message come back as previews, only for readers who may browse the issue.
func TestChatIssuePreviews(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login page", "type": "task"})
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		url := "/api/v1/conversations/" + ch["id"].(string) + "/messages"
		code, m := lead.do("POST", url, map[string]string{"body": "KYB-1 is ready, KYB-99 is not"})
		expect(t, code, 201, m)
		if ps := m["issues"].([]any); len(ps) != 1 || ps[0].(map[string]any)["title"] != "Login page" || ps[0].(map[string]any)["key"] != "KYB-1" ||
			ps[0].(map[string]any)["status"] == "" {
			t.Fatalf("lead preview = %v", m)
		}
		_, page := dev.do("GET", url, nil)
		if ps := page["items"].([]any)[0].(map[string]any)["issues"].([]any); len(ps) != 0 {
			t.Fatalf("dev is not in KYB, yet sees %v", ps)
		}
	})
}

// S54a: pinned messages and search across the conversations one may read.
func TestChatPinsAndSearch(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, pub := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		_, priv := lead.do("POST", "/api/v1/channels", map[string]any{"name": "secret", "private": true})
		pubURL := "/api/v1/conversations/" + pub["id"].(string)
		dev.do("POST", pubURL+"/join", nil)
		_, m := lead.do("POST", pubURL+"/messages", map[string]string{"body": "Release checklist v2"})
		lead.do("POST", "/api/v1/conversations/"+priv["id"].(string)+"/messages", map[string]string{"body": "release salaries"})

		code, p := dev.do("POST", "/api/v1/messages/"+m["id"].(string)+"/pin", nil)
		expect(t, code, 200, p)
		if p["pinned_by"] == nil || p["pinned_at"] == nil {
			t.Fatalf("pinned = %v", p)
		}
		_, pins := lead.do("GET", pubURL+"/pins", nil)
		if items := pins["items"].([]any); len(items) != 1 || items[0].(map[string]any)["body"] != "Release checklist v2" {
			t.Fatalf("pins = %v", pins)
		}
		code, p = dev.do("DELETE", "/api/v1/messages/"+m["id"].(string)+"/pin", nil)
		expect(t, code, 200, p)
		if p["pinned_by"] != nil {
			t.Fatalf("unpinned = %v", p)
		}

		_, found := dev.do("GET", "/api/v1/chat/search?q=RELEASE", nil)
		if items := found["items"].([]any); len(items) != 1 || items[0].(map[string]any)["conversation_id"] != pub["id"] {
			t.Fatalf("dev search = %v (the private channel stays private)", found)
		}
		_, found = lead.do("GET", "/api/v1/chat/search?q=release", nil)
		if len(found["items"].([]any)) != 2 {
			t.Fatalf("lead search = %v", found)
		}
		code, b := dev.do("GET", "/api/v1/chat/search?q=r", nil)
		expect(t, code, 422, b)
	})
}
