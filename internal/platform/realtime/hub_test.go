package realtime_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/realtime"
)

func recv(t *testing.T, ch <-chan realtime.Event) realtime.Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("subscription closed")
		}
		return e
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
	return realtime.Event{}
}

func none(t *testing.T, ch <-chan realtime.Event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event %+v", e)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestPublishReachesOnlyAddressees(t *testing.T) {
	h := realtime.NewHub(8)
	a, stopA := h.Subscribe("w1", "alice", 0)
	defer stopA()
	b, stopB := h.Subscribe("w1", "bob", 0)
	defer stopB()
	other, stopO := h.Subscribe("w2", "alice", 0)
	defer stopO()

	if err := h.Publish("w1", []string{"alice"}, "message.posted", map[string]string{"body": "hi"}); err != nil {
		t.Fatal(err)
	}
	e := recv(t, a)
	if e.Type != "message.posted" || string(e.Data) != `{"body":"hi"}` || e.ID == 0 {
		t.Fatalf("event = %+v", e)
	}
	none(t, b)
	none(t, other) // the same user in another workspace hears nothing
}

func TestReplayAfterLastEventID(t *testing.T) {
	h := realtime.NewHub(3)
	for i := range 5 {
		_ = h.Publish("w1", []string{"alice", "bob"}, "n", i)
	}
	_ = h.Publish("w1", []string{"bob"}, "n", "bob only")
	// The ring keeps the last 3 events (ids 4, 5, 6); alice reconnects after id 4.
	a, stop := h.Subscribe("w1", "alice", 4)
	defer stop()
	if e := recv(t, a); e.ID != 5 || string(e.Data) != "4" {
		t.Fatalf("replayed = %+v", e)
	}
	none(t, a)
}

func TestSlowSubscriberIsClosed(t *testing.T) {
	h := realtime.NewHub(8)
	ch, stop := h.Subscribe("w1", "alice", 0)
	defer stop()
	for i := range realtime.Buffer + 1 {
		_ = h.Publish("w1", []string{"alice"}, "n", i)
	}
	n := 0
	for range ch {
		n++
	}
	if n != realtime.Buffer {
		t.Fatalf("got %d events before close (client reconnects with Last-Event-ID)", n)
	}
}

func TestOnline(t *testing.T) {
	h := realtime.NewHub(8)
	_, stop1 := h.Subscribe("w1", "alice", 0)
	_, stop2 := h.Subscribe("w1", "alice", 0) // two tabs
	_, stop3 := h.Subscribe("w2", "bob", 0)
	defer stop3()
	if h.Connections() != 3 {
		t.Fatalf("connections = %d", h.Connections())
	}
	if got := h.Online("w1", []string{"alice", "bob"}); !slices.Equal(got, []string{"alice"}) {
		t.Fatalf("online = %v", got)
	}
	stop1()
	stop1() // idempotent
	if got := h.Online("w1", []string{"alice"}); len(got) != 1 {
		t.Fatalf("one tab left = %v", got)
	}
	stop2()
	if got := h.Online("w1", []string{"alice"}); len(got) != 0 {
		t.Fatalf("all tabs closed = %v", got)
	}
}

// fakeSink records what the hub forwards (the MQTT broker in production).
type fakeSink struct {
	sent   []string
	online []string
}

func (f *fakeSink) PublishTo(ws string, users []string, typ string, data []byte) {
	f.sent = append(f.sent, ws+"|"+strings.Join(users, ",")+"|"+typ+"|"+string(data))
}

func (f *fakeSink) Online(_ string, users []string) []string {
	var out []string
	for _, u := range users {
		if slices.Contains(f.online, u) {
			out = append(out, u)
		}
	}
	return out
}

// S88: every event also goes to the users' MQTT topics; presence counts MQTT clients too.
func TestHubForwardsToSink(t *testing.T) {
	h := realtime.NewHub(8)
	sink := &fakeSink{online: []string{"bob"}}
	h.Forward(sink)
	_, stop := h.Subscribe("w1", "ann", 0)
	defer stop()
	if err := h.Publish("w1", []string{"ann", "bob"}, "chat.message", map[string]string{"body": "hi"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.sent) != 1 || sink.sent[0] != `w1|ann,bob|chat.message|{"body":"hi"}` {
		t.Fatalf("sent = %v", sink.sent)
	}
	if got := h.Online("w1", []string{"carl", "bob", "ann"}); strings.Join(got, ",") != "bob,ann" {
		t.Fatalf("online = %v", got)
	}
}
