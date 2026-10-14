package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/bakhod1r/kyber/internal/platform/live"
)

// subscribeLive connects to the caller's MQTT-over-WebSocket endpoint and subscribes to every
// project of the workspace; it returns the hint payloads.
func (c *client) subscribeLive(t *testing.T) <-chan string {
	t.Helper()
	got := make(chan string, 100)
	opts := paho.NewClientOptions().AddBroker("ws" + strings.TrimPrefix(c.srv.URL, "http") + "/api/v1/mqtt").
		SetClientID("test").SetAutoReconnect(false).SetConnectTimeout(5 * time.Second).
		SetHTTPHeaders(http.Header{"Authorization": {"Bearer " + c.token}})
	m := paho.NewClient(opts)
	if tok := m.Connect(); !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("connect: %v", tok.Error())
	}
	t.Cleanup(func() { m.Disconnect(10) })
	tok := m.Subscribe(live.Topic("+", "+"), 0, func(_ paho.Client, msg paho.Message) { got <- string(msg.Payload()) })
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe: %v", tok.Error())
	}
	return got
}

// Real time (ADR-0013): members get a hint naming the project and the event — never its
// content — and others in the workspace get nothing.
func TestLiveHints(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		code, b := anon.do("GET", "/api/v1/mqtt", nil)
		expect(t, code, 401, b)

		mine, theirs := lead.subscribeLive(t), out.subscribeLive(t)
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret plan", "type": "task"})
		select {
		case l := <-mine:
			if l != `{"project":"KYB","event":"issue.created"}` {
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
