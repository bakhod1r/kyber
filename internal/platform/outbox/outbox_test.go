package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type ev struct{ N string }

func (e ev) EventName() string { return e.N }

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRelayDispatchesInOrderOnce(t *testing.T) {
	ctx := context.Background()
	var a, b []outbox.Event
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return a }), stamped(func() []outbox.Event { return b }))
	var got []string
	relay := outbox.NewRelay(store, quiet())
	relay.Handle("x.created", func(_ context.Context, m outbox.Message) error {
		if m.At.IsZero() {
			t.Error("message without time")
		}
		got = append(got, m.Name+":"+string(m.Payload))
		return nil
	})
	a = append(a, ev{"x.created"}, ev{"y.ignored"})
	b = append(b, ev{"x.created"})

	if n, err := relay.ProcessOnce(ctx); err != nil || n != 3 {
		t.Fatalf("first pass = %d, %v", n, err)
	}
	if n, _ := relay.ProcessOnce(ctx); n != 0 {
		t.Fatalf("second pass re-delivered %d events", n)
	}
	if strings.Join(got, "|") != `x.created:{"N":"x.created"}|x.created:{"N":"x.created"}` {
		t.Fatalf("handled = %v", got)
	}
	a = append(a, ev{"x.created"})
	if n, _ := relay.ProcessOnce(ctx); n != 1 || len(got) != 3 {
		t.Fatalf("new event: n=%d handled=%d", n, len(got))
	}
}

func TestRelayRetriesFailedEventsAndKeepsOrder(t *testing.T) {
	ctx := context.Background()
	src := []outbox.Event{ev{"e"}, ev{"e"}, ev{"e"}}
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return src }))
	var seen []int64
	fail := true
	relay := outbox.NewRelay(store, quiet())
	relay.Handle("e", func(_ context.Context, m outbox.Message) error {
		seen = append(seen, m.ID)
		if m.ID == 2 && fail {
			return errors.New("transient")
		}
		return nil
	})
	if n, err := relay.ProcessOnce(ctx); err == nil || n != 1 {
		t.Fatalf("failing pass = %d, %v (want 1 published, error)", n, err)
	}
	fail = false
	if n, err := relay.ProcessOnce(ctx); err != nil || n != 2 {
		t.Fatalf("retry pass = %d, %v", n, err)
	}
	// Event 3 must not run before event 2 succeeded.
	if got := seen; len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 2 || got[3] != 3 {
		t.Fatalf("delivery sequence = %v", got)
	}
}

func TestRelayRunStopsOnCancel(t *testing.T) {
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return []outbox.Event{ev{"e"}} }))
	relay := outbox.NewRelay(store, quiet())
	done := make(chan struct{})
	handled := make(chan struct{}, 1)
	relay.Handle("e", func(context.Context, outbox.Message) error {
		select {
		case handled <- struct{}{}:
		default:
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { relay.Run(ctx, 5_000_000); close(done) }() // 5ms
	<-handled
	cancel()
	<-done
}

// stamped stamps every event with t0, as a repository would at write time.
func stamped(log func() []outbox.Event) outbox.Source {
	return func() ([]outbox.Event, []time.Time) {
		events := log()
		at := make([]time.Time, len(events))
		for i := range at {
			at[i] = t0
		}
		return events, at
	}
}

var t0 = time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)

// QA-2: a message carries the time its event was recorded, not when the relay saw it.
func TestMemoryStoreKeepsRecordTime(t *testing.T) {
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return []outbox.Event{ev{"e"}} }))
	var got time.Time
	_, _ = store.Process(context.Background(), 10, func(m outbox.Message) error { got = m.At; return nil })
	if !got.Equal(t0) {
		t.Fatalf("At = %s, want the recording time %s", got, t0)
	}
}
