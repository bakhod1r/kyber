package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
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
		// S64: reading the conversation reads its mentions.
		code, b := qa.do("POST", "/api/v1/notifications/read-all?conversation_id="+pub["id"].(string), nil)
		expect(t, code, 204, b)
		if _, after := qa.do("GET", "/api/v1/notifications", nil); after["unread"] != float64(0) {
			t.Fatalf("after reading #general = %v", after)
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

// upload sends raw bytes as a file (POST /conversations/{id}/files) with the upload header; cookie, when set,
// authenticates instead of the client's token.
func (c *client) chatUpload(conv, name string, body []byte, header bool, cookie *http.Cookie) (int, map[string]any) {
	c.t.Helper()
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/v1/conversations/"+conv+"/files?name="+url.QueryEscape(name), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	if header {
		req.Header.Set("X-Kyber-Request", "1")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
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

// S54b: files in chat — stored in the blob store, served only to readers, with safe headers.
func TestChatFiles(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "design", "private": true})
		conv := ch["id"].(string)
		png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 200)...)

		code, m := lead.chatUpload(conv, "../mock up.png", png, true, nil)
		expect(t, code, 201, m)
		f := m["file"].(map[string]any)
		if f["name"] != "mock up.png" || f["type"] != "image/png" || f["size"] != float64(len(png)) || m["body"] != "mock up.png" {
			t.Fatalf("file message = %v", m)
		}
		res := lead.raw("GET", f["url"].(string), nil, nil)
		got, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 || !bytes.Equal(got, png) || res.Header.Get("Content-Type") != "image/png" ||
			res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "inline") ||
			!strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Fatalf("download = %d %v", res.StatusCode, res.Header)
		}
		res = dev.raw("GET", f["url"].(string), nil, nil)
		res.Body.Close()
		expect(t, res.StatusCode, 404, nil)
		code, b := dev.chatUpload(conv, "x.txt", []byte("x"), true, nil)
		expect(t, code, 404, b)

		// An HTML file is downloaded, never rendered in Kyber's origin.
		code, h := lead.chatUpload(conv, "page.html", []byte("<html><script>alert(1)</script></html>"), true, nil)
		expect(t, code, 201, h)
		res = lead.raw("GET", h["file"].(map[string]any)["url"].(string), nil, nil)
		res.Body.Close()
		if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
			t.Fatalf("html disposition = %v", res.Header)
		}

		// Cookie sessions must send the upload header (CSRF: it forces a CORS preflight cross-site).
		res = anon.raw("POST", "/api/v1/auth/login", map[string]string{"email": "lead@x.uz", "password": "long enough pw"}, nil)
		res.Body.Close()
		var ck *http.Cookie
		for _, x := range res.Cookies() {
			if x.Name == "kyber_session" {
				ck = x
			}
		}
		code, b = lead.chatUpload(conv, "a.txt", []byte("hello"), false, ck)
		expect(t, code, 403, b)
		code, b = lead.chatUpload(conv, "a.txt", []byte("hello"), true, ck)
		expect(t, code, 201, b)

		code, b = lead.chatUpload(conv, "big.bin", bytes.Repeat([]byte{1}, 25<<20+1), true, nil)
		expect(t, code, 422, b)
		code, b = lead.do("DELETE", "/api/v1/messages/"+m["id"].(string), nil)
		expect(t, code, 204, b)
		res = lead.raw("GET", f["url"].(string), nil, nil)
		res.Body.Close()
		expect(t, res.StatusCode, 404, nil)
	})
}

// S65: save a message for later; its reminder arrives as a notification at its time.
func TestSavedForLater(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		conv := ch["id"].(string)
		dev.do("POST", "/api/v1/conversations/"+conv+"/join", nil)
		_, m := lead.do("POST", "/api/v1/conversations/"+conv+"/messages", map[string]string{"body": "check the deploy plan"})
		mid := m["id"].(string)

		code, b := dev.do("PUT", "/api/v1/messages/"+mid+"/save", map[string]any{"remind_at": time.Now().Add(-time.Minute).Format(time.RFC3339)})
		expect(t, code, 422, b)
		code, b = dev.do("PUT", "/api/v1/messages/"+mid+"/save", map[string]any{"remind_at": time.Now().Add(1500 * time.Millisecond).Format(time.RFC3339Nano)})
		expect(t, code, 200, b)
		_, list := dev.do("GET", "/api/v1/chat/saved", nil)
		items := list["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["message"].(map[string]any)["body"] != "check the deploy plan" {
			t.Fatalf("saved = %v", list)
		}
		if _, other := lead.do("GET", "/api/v1/chat/saved", nil); len(other["items"].([]any)) != 0 {
			t.Fatalf("lead's saved = %v", other)
		}
		eventuallyJSON(t, dev, "/api/v1/notifications", func(b map[string]any) bool {
			for _, n := range b["items"].([]any) {
				if n := n.(map[string]any); n["kind"] == "reminder" && n["conversation_id"] == conv && n["excerpt"] == "check the deploy plan" {
					return true
				}
			}
			return false
		})
		code, b = dev.do("DELETE", "/api/v1/messages/"+mid+"/save", nil)
		expect(t, code, 204, b)
		if _, after := dev.do("GET", "/api/v1/chat/saved", nil); len(after["items"].([]any)) != 0 {
			t.Fatalf("after unsave = %v", after)
		}
	}, server.WithReminderEvery(100*time.Millisecond))
}

// S66: status and Do Not Disturb, seen by the workspace.
func TestStatusAndDND(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		_, me := lead.do("GET", "/api/v1/me", nil)
		dev := anon.signedIn("dev@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		code, b := lead.do("PUT", "/api/v1/me/status", map[string]any{"emoji": "🏖", "text": "On vacation",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339), "dnd_until": time.Now().Add(30 * time.Minute).Format(time.RFC3339)})
		expect(t, code, 200, b)
		code, b = lead.do("PUT", "/api/v1/me/status", map[string]any{"text": strings.Repeat("x", 101)})
		expect(t, code, 422, b)
		code, list := dev.do("GET", "/api/v1/chat/statuses?user_id="+me["id"].(string)+"&user_id="+devMe["id"].(string), nil)
		expect(t, code, 200, list)
		items := list["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["text"] != "On vacation" || items[0].(map[string]any)["dnd"] != true {
			t.Fatalf("statuses = %v", list)
		}
		code, b = lead.do("PUT", "/api/v1/me/status", map[string]any{"emoji": "", "text": ""})
		expect(t, code, 200, b)
		if _, l := dev.do("GET", "/api/v1/chat/statuses?user_id="+me["id"].(string), nil); len(l["items"].([]any)) != 0 {
			t.Fatalf("after clearing = %v", l)
		}
	})
}

// S67: a scheduled message is posted at its time; /remind me arrives as a notification.
func TestScheduledMessagesAndReminders(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		conv := ch["id"].(string)
		at := func(d time.Duration) string { return time.Now().Add(d).Format(time.RFC3339Nano) }

		code, b := lead.do("POST", "/api/v1/conversations/"+conv+"/scheduled", map[string]any{"body": "too soon", "send_at": at(-time.Minute)})
		expect(t, code, 422, b)
		code, sc := lead.do("POST", "/api/v1/conversations/"+conv+"/scheduled", map[string]any{"body": "draft", "send_at": at(time.Hour)})
		expect(t, code, 201, sc)
		code, b = dev.do("PATCH", "/api/v1/scheduled/"+sc["id"].(string), map[string]any{"body": "x", "send_at": at(time.Hour)})
		expect(t, code, 404, b)
		code, b = lead.do("PATCH", "/api/v1/scheduled/"+sc["id"].(string), map[string]any{"body": "standup moved to 11", "send_at": at(1500 * time.Millisecond)})
		expect(t, code, 200, b)
		if _, l := lead.do("GET", "/api/v1/chat/scheduled", nil); len(l["items"].([]any)) != 1 {
			t.Fatalf("scheduled = %v", l)
		}
		if _, l := dev.do("GET", "/api/v1/chat/scheduled", nil); len(l["items"].([]any)) != 0 {
			t.Fatalf("dev sees lead's scheduled = %v", l)
		}
		eventuallyJSON(t, lead, "/api/v1/conversations/"+conv+"/messages", func(b map[string]any) bool {
			items := b["items"].([]any)
			return len(items) == 1 && items[0].(map[string]any)["body"] == "standup moved to 11"
		})
		if _, l := lead.do("GET", "/api/v1/chat/scheduled", nil); len(l["items"].([]any)) != 0 {
			t.Fatalf("still scheduled after sending = %v", l)
		}
		_, gone := lead.do("POST", "/api/v1/conversations/"+conv+"/scheduled", map[string]any{"body": "never mind", "send_at": at(time.Hour)})
		code, b = dev.do("DELETE", "/api/v1/scheduled/"+gone["id"].(string), nil)
		expect(t, code, 404, b)
		code, b = lead.do("DELETE", "/api/v1/scheduled/"+gone["id"].(string), nil)
		expect(t, code, 204, b)

		code, b = lead.do("POST", "/api/v1/conversations/"+conv+"/reminders", map[string]any{"text": "call the bank", "at": at(time.Second)})
		expect(t, code, 201, b)
		eventuallyJSON(t, lead, "/api/v1/notifications", func(b map[string]any) bool {
			for _, n := range b["items"].([]any) {
				if n := n.(map[string]any); n["kind"] == "reminder" && n["excerpt"] == "call the bank" {
					return true
				}
			}
			return false
		})
	}, server.WithReminderEvery(100*time.Millisecond))
}

// S68: read receipts in DMs.
func TestReadReceipts(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, dm := lead.do("POST", "/api/v1/dms", map[string]any{"user_ids": []string{devMe["id"].(string)}})
		conv := dm["id"].(string)
		lead.do("POST", "/api/v1/conversations/"+conv+"/messages", map[string]string{"body": "hi"})
		_, before := lead.do("GET", "/api/v1/conversations/"+conv+"/receipts", nil)
		for _, r := range before["items"].([]any) {
			if r.(map[string]any)["user_id"] == devMe["id"] {
				t.Fatalf("dev has not read yet: %v", before)
			}
		}
		code, b := dev.do("POST", "/api/v1/conversations/"+conv+"/read", nil)
		expect(t, code, 204, b)
		_, after := lead.do("GET", "/api/v1/conversations/"+conv+"/receipts", nil)
		found := false
		for _, r := range after["items"].([]any) {
			found = found || r.(map[string]any)["user_id"] == devMe["id"]
		}
		if !found {
			t.Fatalf("receipts = %v", after)
		}
		code, b = anon.signedIn("qa@x.uz").do("GET", "/api/v1/conversations/"+conv+"/receipts", nil)
		expect(t, code, 404, b)
	})
}

// S69: owners and admins manage a channel; archived channels are read-only.
func TestChannelManagement(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, qaMe := qa.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		lead.do("POST", "/api/v1/channels", map[string]any{"name": "taken"})
		id := ch["id"].(string)
		dev.do("POST", "/api/v1/conversations/"+id+"/join", nil)
		qa.do("POST", "/api/v1/conversations/"+id+"/join", nil)

		code, b := dev.do("PATCH", "/api/v1/conversations/"+id, map[string]any{"name": "team"})
		expect(t, code, 403, b)
		code, b = lead.do("PUT", "/api/v1/conversations/"+id+"/admins/"+devMe["id"].(string), nil)
		expect(t, code, 200, b)
		if b["admin_ids"].([]any)[0] != devMe["id"] {
			t.Fatalf("admins = %v", b)
		}
		code, b = dev.do("PATCH", "/api/v1/conversations/"+id, map[string]any{"name": "taken"})
		expect(t, code, 409, b)
		code, b = dev.do("PATCH", "/api/v1/conversations/"+id, map[string]any{"name": "team", "topic": "The team"})
		expect(t, code, 200, b)
		if b["name"] != "team" || b["topic"] != "The team" {
			t.Fatalf("renamed = %v", b)
		}
		code, b = dev.do("DELETE", "/api/v1/conversations/"+id+"/members/"+qaMe["id"].(string), nil)
		expect(t, code, 200, b)
		code, b = qa.do("POST", "/api/v1/conversations/"+id+"/messages", map[string]string{"body": "hi"})
		expect(t, code, 403, b)
		code, b = dev.do("POST", "/api/v1/conversations/"+id+"/archive", nil)
		expect(t, code, 200, b)
		if b["archived"] != true {
			t.Fatalf("archived = %v", b)
		}
		code, b = lead.do("POST", "/api/v1/conversations/"+id+"/messages", map[string]string{"body": "anyone?"})
		expect(t, code, 403, b)
		code, b = lead.do("POST", "/api/v1/conversations/"+id+"/unarchive", nil)
		expect(t, code, 200, b)
		code, b = dev.do("DELETE", "/api/v1/conversations/"+id+"/admins/"+devMe["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", "/api/v1/conversations/"+id+"/admins/"+devMe["id"].(string), nil)
		expect(t, code, 200, b)
		if len(b["admin_ids"].([]any)) != 0 {
			t.Fatalf("admins after removal = %v", b)
		}
	})
}

// S70: user groups; @handle in a message notifies the group's members.
func TestUserGroups(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, qaMe := qa.do("GET", "/api/v1/me", nil)
		code, g := lead.do("POST", "/api/v1/groups", map[string]any{"handle": "backend", "name": "Backend", "member_ids": []string{devMe["id"].(string)}})
		expect(t, code, 201, g)
		code, b := dev.do("POST", "/api/v1/groups", map[string]any{"handle": "backend", "member_ids": []string{}})
		expect(t, code, 409, b)
		code, b = dev.do("POST", "/api/v1/groups", map[string]any{"handle": "here", "member_ids": []string{}})
		expect(t, code, 422, b)
		code, b = qa.do("PATCH", "/api/v1/groups/"+g["id"].(string), map[string]any{"handle": "backend", "name": "Backend",
			"member_ids": []string{devMe["id"].(string), qaMe["id"].(string)}})
		expect(t, code, 200, b)
		if _, l := dev.do("GET", "/api/v1/groups", nil); len(l["items"].([]any)) != 1 {
			t.Fatalf("groups = %v", l)
		}

		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		dev.do("POST", "/api/v1/conversations/"+ch["id"].(string)+"/join", nil)
		lead.do("POST", "/api/v1/conversations/"+ch["id"].(string)+"/messages", map[string]string{"body": "@backend please review"})
		for _, c := range []*client{dev, qa} {
			eventuallyJSON(t, c, "/api/v1/notifications", func(b map[string]any) bool { return len(b["items"].([]any)) == 1 })
		}

		code, b = dev.do("DELETE", "/api/v1/groups/"+g["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", "/api/v1/groups/"+g["id"].(string), nil)
		expect(t, code, 204, b)
		code, b = lead.do("PATCH", "/api/v1/groups/"+g["id"].(string), map[string]any{"handle": "x2", "member_ids": []string{}})
		expect(t, code, 404, b)
	})
}

// S71: notification levels and sidebar sections per person.
func TestConversationPrefs(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		id := ch["id"].(string)
		dev.do("POST", "/api/v1/conversations/"+id+"/join", nil)
		qa.do("POST", "/api/v1/conversations/"+id+"/join", nil)
		code, b := dev.do("PUT", "/api/v1/conversations/"+id+"/prefs", map[string]any{"level": "all", "section": "Work"})
		expect(t, code, 200, b)
		code, b = qa.do("PUT", "/api/v1/conversations/"+id+"/prefs", map[string]any{"level": "muted"})
		expect(t, code, 200, b)
		code, b = qa.do("PUT", "/api/v1/conversations/"+id+"/prefs", map[string]any{"level": "loud"})
		expect(t, code, 422, b)
		if _, l := dev.do("GET", "/api/v1/chat/prefs", nil); len(l["items"].([]any)) != 1 || l["items"].([]any)[0].(map[string]any)["section"] != "Work" {
			t.Fatalf("dev's prefs = %v", l)
		}
		lead.do("POST", "/api/v1/conversations/"+id+"/messages", map[string]string{"body": "deploying now @qa@x.uz"})
		eventuallyJSON(t, dev, "/api/v1/notifications", func(b map[string]any) bool {
			items := b["items"].([]any)
			return len(items) == 1 && items[0].(map[string]any)["kind"] == "message"
		})
		time.Sleep(300 * time.Millisecond)
		if _, n := qa.do("GET", "/api/v1/notifications", nil); len(n["items"].([]any)) != 0 {
			t.Fatalf("muted qa notified: %v", n)
		}
	})
}

// S73: a channel linked to a project gets its news; outsiders see the key, not the details.
func TestProjectChannels(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "kyb-news"})
		id := ch["id"].(string)
		dev.do("POST", "/api/v1/conversations/"+id+"/join", nil)
		_, devCh := dev.do("POST", "/api/v1/channels", map[string]any{"name": "dev-room"})
		code, b := dev.do("PUT", "/api/v1/conversations/"+devCh["id"].(string)+"/project", map[string]string{"project_key": "KYB"})
		expect(t, code, 403, b)
		code, b = lead.do("PUT", "/api/v1/conversations/"+id+"/project", map[string]string{"project_key": "KYB"})
		expect(t, code, 200, b)
		if b["project_key"] != "KYB" {
			t.Fatalf("linked = %v", b)
		}
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret roadmap", "type": "task"})
		eventuallyJSON(t, lead, "/api/v1/conversations/"+id+"/messages", func(b map[string]any) bool {
			items := b["items"].([]any)
			if len(items) != 1 {
				return false
			}
			m := items[0].(map[string]any)
			return m["kind"] == "project_event" && m["body"] == "created KYB-1" && len(m["issues"].([]any)) == 1
		})
		_, page := dev.do("GET", "/api/v1/conversations/"+id+"/messages", nil)
		m := page["items"].([]any)[0].(map[string]any)
		if len(m["issues"].([]any)) != 0 || strings.Contains(m["body"].(string), "Secret") {
			t.Fatalf("dev (not in KYB) sees details: %v", m)
		}
	})
}

// S79: bookmarks in a conversation's header.
func TestBookmarks(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		url := "/api/v1/conversations/" + ch["id"].(string) + "/bookmarks"
		code, b := lead.do("POST", url, map[string]string{"kind": "link", "ref": "javascript:alert(1)", "title": "x"})
		expect(t, code, 422, b)
		code, bm := lead.do("POST", url, map[string]string{"kind": "link", "ref": "https://ci.example", "title": "CI"})
		expect(t, code, 201, bm)
		code, b = dev.do("POST", url, map[string]string{"kind": "doc", "ref": "abc", "title": "Design"})
		expect(t, code, 403, b) // not a member yet
		_, list := dev.do("GET", url, nil)
		if items := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "CI" {
			t.Fatalf("bookmarks = %v", list)
		}
		code, b = dev.do("DELETE", "/api/v1/bookmarks/"+bm["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", "/api/v1/bookmarks/"+bm["id"].(string), nil)
		expect(t, code, 204, b)
	})
}

// S80: channel workflows — a welcome when someone joins; schedules are validated and listed.
func TestWorkflows(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		url := "/api/v1/conversations/" + ch["id"].(string) + "/workflows"
		code, b := dev.do("POST", url, map[string]any{"trigger": "member_joined", "text": "hi"})
		expect(t, code, 403, b)
		code, b = lead.do("POST", url, map[string]any{"trigger": "schedule", "minute": 9 * 60, "days": []int{}, "text": "x"})
		expect(t, code, 422, b)
		code, sched := lead.do("POST", url, map[string]any{"trigger": "schedule", "minute": 9*60 + 30, "days": []int{1, 2, 3, 4, 5}, "text": "Standup"})
		expect(t, code, 201, sched)
		code, b = lead.do("POST", url, map[string]any{"trigger": "member_joined", "text": "Welcome {name}!"})
		expect(t, code, 201, b)
		dev.do("POST", "/api/v1/conversations/"+ch["id"].(string)+"/join", nil)
		_, page := lead.do("GET", "/api/v1/conversations/"+ch["id"].(string)+"/messages", nil)
		if items := page["items"].([]any); len(items) != 1 || items[0].(map[string]any)["kind"] != "workflow" ||
			items[0].(map[string]any)["body"] != "Welcome dev@x.uz!" && !strings.HasPrefix(items[0].(map[string]any)["body"].(string), "Welcome ") {
			t.Fatalf("welcome = %v", page)
		}
		if _, l := dev.do("GET", url, nil); len(l["items"].([]any)) != 2 {
			t.Fatalf("workflows = %v", l)
		}
		code, b = dev.do("DELETE", "/api/v1/workflows/"+sched["id"].(string), nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", "/api/v1/workflows/"+sched["id"].(string), nil)
		expect(t, code, 204, b)
	})
}
