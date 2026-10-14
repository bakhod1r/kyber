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
	return c.subscribeMQTT(t, live.Topic("+", "+"))
}

// subscribeMQTT subscribes the caller to an MQTT topic filter and returns the payloads.
func (c *client) subscribeMQTT(t *testing.T, filter string) <-chan string {
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
	tok := m.Subscribe(filter, 0, func(_ paho.Client, msg paho.Message) { got <- string(msg.Payload()) })
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

// S88: chat events reach the conversation's members on their own MQTT topic (in place of SSE); others
// in the workspace get nothing, and presence counts MQTT clients.
func TestChatOverMQTT(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		out := anon.signedIn("out@x.uz")
		_, ch := lead.do("POST", "/api/v1/channels", map[string]any{"name": "general"})
		mine, theirs := lead.subscribeMQTT(t, live.UserTopic("+", "+")), out.subscribeMQTT(t, live.UserTopic("+", "+"))
		_, me := lead.do("GET", "/api/v1/me", nil)
		code, b := out.do("GET", "/api/v1/presence?user_id="+me["id"].(string), nil)
		expect(t, code, 200, b)
		if on := b["online"].([]any); len(on) != 1 {
			t.Fatalf("presence = %v", b)
		}
		lead.do("POST", "/api/v1/conversations/"+ch["id"].(string)+"/messages", map[string]string{"body": "hello mqtt"})
		deadline := time.After(10 * time.Second)
		for found := false; !found; {
			select {
			case l := <-mine:
				found = strings.Contains(l, `"type":"message.posted"`) && strings.Contains(l, "hello mqtt")
			case <-deadline:
				t.Fatal("no chat event over MQTT")
			}
		}
		select {
		case l := <-theirs:
			t.Fatalf("outsider got %s", l)
		case <-time.After(1500 * time.Millisecond):
		}
	})
}
