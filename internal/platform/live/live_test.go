package live_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/live"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func TestFromMessage(t *testing.T) {
	for _, tc := range []struct {
		payload string
		want    string
		ok      bool
	}{
		{`{"key":"KYB-12","title":"secret"}`, "KYB", true},
		{`{"key":"MY-APP-3"}`, "MY-APP", true},
		{`{"project":"OPS","id":"s-1"}`, "OPS", true},
		{`{"id":"x"}`, "", false},
		{`{"key":"nodash"}`, "", false},
		{`not json`, "", false},
	} {
		c, ok := live.FromMessage(outbox.Message{Name: "e", Workspace: "w-1", Payload: []byte(tc.payload)})
		if ok != tc.ok || c.Project != tc.want || (ok && (c.Workspace != "w-1" || c.Event != "e")) {
			t.Errorf("%s = %+v %v", tc.payload, c, ok)
		}
	}
}

// stream opens the SSE handler as user in workspace ws and returns its lines.
func stream(t *testing.T, h http.Handler, ws, user string) (<-chan string, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(auth.WithActor(tenant.With(r.Context(), ws), user)))
	}))
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %q", ct)
	}
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return lines, func() { cancel(); res.Body.Close(); srv.Close() }
}

// next waits for a line with prefix.
func next(t *testing.T, lines <-chan string, prefix string) string {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("stream ended waiting for %q", prefix)
			}
			if strings.HasPrefix(l, prefix) {
				return l
			}
		case <-timeout:
			t.Fatalf("no %q line", prefix)
		}
	}
}

func waitSubscribers(t *testing.T, h *live.Hub, n int) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); h.Subscribers() != n; {
		if time.Now().After(deadline) {
			t.Fatalf("subscribers = %d, want %d", h.Subscribers(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHandlerStreamsOnlyBrowsableProjects(t *testing.T) {
	hub := live.NewHub()
	can := func(_ context.Context, user, project string) bool { return project == "KYB" || user == "admin" }
	h := hub.Handler(can, 20*time.Millisecond)
	lines, stop := stream(t, h, "w-1", "dev")
	defer stop()
	next(t, lines, ": connected")
	waitSubscribers(t, hub, 1)

	_ = hub.Broadcast(context.Background(), live.Change{Workspace: "w-1", Project: "SECRET", Event: "issue.created"})
	_ = hub.Broadcast(context.Background(), live.Change{Workspace: "w-2", Project: "KYB", Event: "issue.created"})
	_ = hub.Broadcast(context.Background(), live.Change{Workspace: "w-1", Project: "KYB", Event: "issue.changed"})
	next(t, lines, "event: change")
	if data := next(t, lines, "data:"); data != `data: {"project":"KYB","event":"issue.changed"}` {
		t.Fatalf("data = %s", data)
	}
	next(t, lines, ": ping")
	stop()
	waitSubscribers(t, hub, 0)
}

func TestSlowSubscriberIsToldToResync(t *testing.T) {
	hub := live.NewHub()
	release := make(chan struct{})
	can := func(context.Context, string, string) bool { <-release; return true }
	lines, stop := stream(t, hub.Handler(can, time.Hour), "w-1", "dev")
	defer stop()
	next(t, lines, ": connected")
	waitSubscribers(t, hub, 1)
	for range 100 { // the handler is stuck in can: the buffer fills and overflows
		hub.Publish(live.Change{Workspace: "w-1", Project: "KYB", Event: "e"})
	}
	close(release)
	next(t, lines, "event: resync")
}

type failing struct{}

func (failing) Broadcast(context.Context, live.Change) error { return errors.New("db down") }

func TestOnEvent(t *testing.T) {
	hub := live.NewHub()
	var logged strings.Builder
	log := slog.New(slog.NewTextHandler(&logged, nil))
	if err := live.OnEvent(failing{}, log)(context.Background(), outbox.Message{Name: "e", Payload: []byte(`{"key":"KYB-1"}`)}); err != nil || !strings.Contains(logged.String(), "db down") {
		t.Fatalf("a failed hint must not fail the event: %v %q", err, logged.String())
	}
	if err := live.OnEvent(hub, log)(context.Background(), outbox.Message{Name: "e", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	_ = io.Discard
}

// noFlush is a writer without Flush: the stream cannot work and ends at once.
type noFlush struct{ http.ResponseWriter }

func TestHandlerNeedsAFlushableConnection(t *testing.T) {
	hub := live.NewHub()
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	hub.Handler(func(context.Context, string, string) bool { return true }, time.Hour).
		ServeHTTP(noFlush{rec}, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	if hub.Subscribers() != 0 {
		t.Fatal("subscriber left behind")
	}
}

// brokenConn fails writes after ok of them succeed, like a client that went away.
type brokenConn struct {
	header      http.Header
	ok          int
	deadlineErr error
}

func (b *brokenConn) Header() http.Header { return b.header }
func (b *brokenConn) WriteHeader(int)     {}
func (b *brokenConn) Write(p []byte) (int, error) {
	if b.ok == 0 {
		return 0, errors.New("broken pipe")
	}
	b.ok--
	return len(p), nil
}
func (b *brokenConn) FlushError() error                { return nil }
func (b *brokenConn) SetWriteDeadline(time.Time) error { return b.deadlineErr }

func TestHandlerStopsWhenTheClientIsGone(t *testing.T) {
	all := func(context.Context, string, string) bool { return true }
	for _, tc := range []struct {
		name     string
		conn     *brokenConn
		publish  int
		lagFirst bool
	}{
		{"deadline", &brokenConn{deadlineErr: errors.New("closed")}, 0, false},
		{"connected", &brokenConn{ok: 0}, 0, false},
		{"ping", &brokenConn{ok: 1}, 0, false},
		{"change", &brokenConn{ok: 1}, 1, false},
		{"resync", &brokenConn{ok: 2}, buffer + 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := live.NewHub()
			tc.conn.header = http.Header{}
			heartbeat := time.Hour
			if tc.name == "ping" {
				heartbeat = time.Millisecond
			}
			can := all
			release, entered := make(chan struct{}), make(chan struct{})
			if tc.lagFirst { // hold the handler in its first check until the buffer overflowed
				first := true
				can = func(context.Context, string, string) bool {
					if first {
						first = false
						close(entered)
						<-release
					}
					return true
				}
			} else {
				close(release)
			}
			done := make(chan struct{})
			go func() {
				hub.Handler(can, heartbeat).ServeHTTP(tc.conn, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(tenant.With(context.Background(), "w-1")))
				close(done)
			}()
			if tc.publish > 0 {
				waitSubscribers(t, hub, 1)
				if tc.lagFirst { // the handler takes one hint and waits in can; then the rest overflow
					hub.Publish(live.Change{Workspace: "w-1", Project: "KYB", Event: "e"})
					<-entered
				}
				for range tc.publish {
					hub.Publish(live.Change{Workspace: "w-1", Project: "KYB", Event: "e"})
				}
				if tc.lagFirst {
					close(release)
				}
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("handler kept running for a gone client")
			}
		})
	}
}

const buffer = 64

// S83: Follow gives the same hints as the handler to another stream (the chat stream carries them).
func TestFollowFiltersByBrowsing(t *testing.T) {
	h := live.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hints := h.Follow(ctx, "u-1", "ws", func(_ context.Context, user, project string) bool { return project == "KYB" })
	for h.Subscribers() == 0 {
		time.Sleep(time.Millisecond)
	}
	h.Publish(live.Change{Workspace: "ws", Project: "SECRET", Event: "issue.created"})
	h.Publish(live.Change{Workspace: "other", Project: "KYB", Event: "issue.created"})
	h.Publish(live.Change{Workspace: "ws", Project: "KYB", Event: "issue.changed"})
	select {
	case got := <-hints:
		if got.Type != "change" || string(got.Data) != `{"project":"KYB","event":"issue.changed"}` {
			t.Fatalf("hint = %+v %s", got, got.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no hint")
	}
	cancel()
	for h.Subscribers() != 0 {
		time.Sleep(time.Millisecond)
	}
}
