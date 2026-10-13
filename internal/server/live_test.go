package server_test

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// openStream opens the caller's live hint stream and returns its data lines.
func (c *client) openStream(t *testing.T) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.srv.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); res.Body.Close() })
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream = %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "data:") || l == ": connected" {
				lines <- l
			}
		}
	}()
	if l := <-lines; l != ": connected" {
		t.Fatalf("first line = %q", l)
	}
	return lines
}

// Real time: members get a hint naming the project and the event — never its content — and
// others in the workspace get nothing.
func TestLiveHints(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		code, b := anon.do("GET", "/api/v1/events", nil)
		expect(t, code, 401, b)

		mine, theirs := lead.openStream(t), out.openStream(t)
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret plan", "type": "task"})
		select {
		case l := <-mine:
			if l != `data: {"project":"KYB","event":"issue.created"}` {
				t.Fatalf("hint = %s", l)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("no hint for the member")
		}
		select {
		case l := <-theirs:
			t.Fatalf("outsider got %s", l)
		case <-time.After(1500 * time.Millisecond):
		}
	})
}
