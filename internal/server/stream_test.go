package server_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

type sse struct{ id, event, data string }

// stream opens GET /api/v1/stream; events arrive on the channel until the test ends.
func (c *client) stream(lastID string) <-chan sse {
	c.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c.t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", c.srv.URL+"/api/v1/stream", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		c.t.Fatalf("stream = %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	spec.exercised(req) // a stream never ends, so its body is not validated
	out := make(chan sse, 32)
	go func() {
		defer res.Body.Close()
		var e sse
		for sc := bufio.NewScanner(res.Body); sc.Scan(); {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "id: "):
				e.id = line[4:]
			case strings.HasPrefix(line, "event: "):
				e.event = line[7:]
			case strings.HasPrefix(line, "data: "):
				e.data = line[6:]
			case line == "" && e.event != "":
				out <- e
				e = sse{}
			}
		}
	}()
	return out
}

func next(t *testing.T, ch <-chan sse, event string) sse {
	t.Helper()
	for {
		select {
		case e := <-ch:
			if e.event == event {
				return e
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %s event", event)
		}
	}
}

func quiet(t *testing.T, ch <-chan sse) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event %+v", e)
	case <-time.After(150 * time.Millisecond):
	}
}

// S51: members hear about messages and typing live; outsiders never do; reconnecting resumes.
func TestChatStream(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		_, devMe := dev.do("GET", "/api/v1/me", nil)
		_, leadMe := lead.do("GET", "/api/v1/me", nil)
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "dev", "private": true})
		url := "/api/v1/conversations/" + ch["id"].(string)
		lead.do("POST", url+"/members", map[string]string{"user_id": devMe["id"].(string)})

		devStream, qaStream := dev.stream(""), qa.stream("")
		_, on := lead.do("GET", "/api/v1/presence?user_id="+devMe["id"].(string)+"&user_id="+leadMe["id"].(string), nil)
		if got := on["online"].([]any); len(got) != 1 || got[0] != devMe["id"] {
			t.Fatalf("presence = %v", on)
		}

		_, m := lead.do("POST", url+"/messages", map[string]string{"body": "deploy at 5"})
		e := next(t, devStream, "message.posted")
		if !strings.Contains(e.data, `"body":"deploy at 5"`) || !strings.Contains(e.data, m["id"].(string)) {
			t.Fatalf("posted = %+v", e)
		}
		code, b := lead.do("POST", url+"/typing", nil)
		expect(t, code, 204, b)
		if e := next(t, devStream, "typing"); !strings.Contains(e.data, leadMe["id"].(string)) {
			t.Fatalf("typing = %+v", e)
		}
		code, b = qa.do("POST", url+"/typing", nil)
		expect(t, code, 404, b)
		lead.do("PATCH", "/api/v1/messages/"+m["id"].(string), map[string]string{"body": "deploy at 6"})
		edited := next(t, devStream, "message.edited")
		quiet(t, qaStream)

		// A tab that reconnects with Last-Event-ID gets what it missed.
		lead.do("DELETE", "/api/v1/messages/"+m["id"].(string), nil)
		if e := next(t, dev.stream(edited.id), "message.deleted"); !strings.Contains(e.data, `"deleted":true`) {
			t.Fatalf("replayed = %+v", e)
		}
	})
}

// quietOf fails if an event of the given type arrives (others, e.g. system messages, are fine).
func quietOf(t *testing.T, ch <-chan sse, event string) {
	t.Helper()
	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case e := <-ch:
			if e.event == event {
				t.Fatalf("unexpected %s %+v", event, e)
			}
		case <-deadline:
			return
		}
	}
}

// S83: board hints (main's /api/v1/events) also arrive on the chat stream, only for projects one may browse.
func TestStreamCarriesBoardHints(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		outsider := anon.signedIn("eve@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		mine, theirs := lead.stream(""), outsider.stream("")
		time.Sleep(200 * time.Millisecond) // both subscribed
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login page", "type": "task"})
		if e := next(t, mine, "change"); !strings.Contains(e.data, `"project":"KYB"`) || !strings.Contains(e.data, `"event":"issue.created"`) {
			t.Fatalf("hint = %+v", e)
		}
		select {
		case e := <-theirs:
			if e.event == "change" {
				t.Fatalf("outsider got a hint: %+v", e)
			}
		case <-time.After(500 * time.Millisecond):
		}
	})
}
