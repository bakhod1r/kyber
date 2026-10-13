package outbox_test

import (
	"context"
	"errors"
	"testing"

	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func TestMetaOfRecordsTheEnvelope(t *testing.T) {
	gotel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("t").Start(auth.WithActor(tenant.With(context.Background(), "w-1"), "u-1"), "request")
	defer span.End()
	m := outbox.MetaOf(ctx, t0)
	want := "00-" + span.SpanContext().TraceID().String() + "-" + span.SpanContext().SpanID().String() + "-01"
	if m.At != t0 || m.Workspace != "w-1" || m.Actor != "u-1" || m.Trace != want {
		t.Fatalf("meta = %+v, want trace %s", m, want)
	}
	if m := outbox.MetaOf(context.Background(), t0); m.Actor != "" || m.Trace != "" {
		t.Fatalf("no actor or span: %+v", m)
	}
}

// metaSource stamps events with a full envelope.
func metaSource(log func() []outbox.Event, m outbox.Meta) outbox.Source {
	return func() ([]outbox.Event, []outbox.Meta) {
		events := log()
		at := make([]outbox.Meta, len(events))
		for i := range at {
			at[i] = m
		}
		return events, at
	}
}

func TestRelayContinuesTheRecordingTrace(t *testing.T) {
	gotel.SetTextMapPropagator(propagation.TraceContext{})
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	gotel.SetTracerProvider(tp)
	t.Cleanup(func() { gotel.SetTracerProvider(noop.NewTracerProvider()) })
	parent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	store := outbox.NewMemoryStore(metaSource(func() []outbox.Event { return []outbox.Event{ev{"e"}} },
		outbox.Meta{At: t0, Workspace: "w-1", Actor: "u-1", Trace: parent}))
	relay := outbox.NewRelay(store, quiet())
	var msg outbox.Message
	var inSpan trace.SpanContext
	relay.Handle("e", func(ctx context.Context, m outbox.Message) error {
		msg, inSpan = m, trace.SpanContextFromContext(ctx)
		return nil
	})
	if _, err := relay.ProcessOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if msg.Actor != "u-1" || msg.Trace != parent || msg.Version != outbox.SchemaVersion {
		t.Fatalf("message = %+v", msg)
	}
	if inSpan.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("handler trace = %s", inSpan.TraceID())
	}
	if spans := exp.GetSpans(); len(spans) != 1 || spans[0].Parent.SpanID().String() != "00f067aa0ba902b7" || spans[0].Name != "outbox e" {
		t.Fatalf("spans = %+v", spans)
	}
}

func TestRelayDeadLettersAPoisonEvent(t *testing.T) {
	ctx := context.Background()
	src := []outbox.Event{ev{"poison"}, ev{"ok"}}
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return src }))
	relay := outbox.NewRelay(store, quiet(), outbox.MaxAttempts(3))
	var ok []int64
	attempts := 0
	relay.Handle("poison", func(context.Context, outbox.Message) error { attempts++; return errors.New("bug") })
	relay.Handle("ok", func(_ context.Context, m outbox.Message) error { ok = append(ok, m.ID); return nil })

	for i := 1; i <= 2; i++ {
		if n, err := relay.ProcessOnce(ctx); err == nil || n != 0 || len(ok) != 0 {
			t.Fatalf("attempt %d: n=%d err=%v ok=%v (the poison event must still block)", i, n, err, ok)
		}
	}
	// The third failure parks it and lets the next event through.
	if n, err := relay.ProcessOnce(ctx); err != nil || n != 1 || len(ok) != 1 || attempts != 3 {
		t.Fatalf("dead-letter pass: n=%d err=%v ok=%v attempts=%d", n, err, ok, attempts)
	}
	dead := store.DeadLetters()
	if len(dead) != 1 || dead[0].Name != "poison" || dead[0].Attempts != 3 || dead[0].LastError == "" {
		t.Fatalf("dead letters = %+v", dead)
	}
	if n, _ := relay.ProcessOnce(ctx); n != 0 || attempts != 3 {
		t.Fatalf("a dead letter is not retried by itself: n=%d attempts=%d", n, attempts)
	}

	// After the fix an operator retries it.
	if err := store.Retry(dead[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Retry(dead[0].ID); !errors.Is(err, outbox.ErrNotDead) {
		t.Fatalf("retry twice: %v", err)
	}
	// A new relay models the deploy that fixed the handler.
	fixed := outbox.NewRelay(store, quiet(), outbox.MaxAttempts(3))
	fixed.Handle("poison", func(_ context.Context, m outbox.Message) error {
		if m.Attempts != 0 {
			t.Errorf("retried message attempts = %d", m.Attempts)
		}
		return nil
	})
	if n, err := fixed.ProcessOnce(ctx); err != nil || n != 1 || len(store.DeadLetters()) != 0 {
		t.Fatalf("retried: n=%d err=%v", n, err)
	}
}

func TestMaxAttemptsDefaultsAndFloor(t *testing.T) {
	src := []outbox.Event{ev{"bad"}}
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return src }))
	relay := outbox.NewRelay(store, quiet(), outbox.MaxAttempts(0)) // below 1 means 1
	relay.Handle("bad", func(context.Context, outbox.Message) error { return errors.New("x") })
	if n, err := relay.ProcessOnce(context.Background()); err != nil || n != 0 || len(store.DeadLetters()) != 1 {
		t.Fatalf("n=%d err=%v dead=%d", n, err, len(store.DeadLetters()))
	}
	if outbox.DefaultMaxAttempts != 10 {
		t.Fatalf("default = %d", outbox.DefaultMaxAttempts)
	}
}

func TestDeadLettersOfAWorkspace(t *testing.T) {
	ctx := context.Background()
	store := outbox.NewMemoryStore(stamped(func() []outbox.Event { return []outbox.Event{ev{"bad"}, ev{"bad"}} }))
	relay := outbox.NewRelay(store, quiet(), outbox.MaxAttempts(1))
	relay.Handle("bad", func(context.Context, outbox.Message) error { return errors.New("x") })
	_, _ = relay.ProcessOnce(ctx)
	if dead, _ := store.DeadLettersOf(ctx, "w-1", 1); len(dead) != 1 {
		t.Fatalf("limit = %v", dead)
	}
	if dead, _ := store.DeadLettersOf(ctx, "w-2", 10); len(dead) != 0 {
		t.Fatalf("other workspace sees %v", dead)
	}
	if err := store.RetryOf(ctx, "w-2", 1); !errors.Is(err, outbox.ErrNotDead) {
		t.Fatalf("other workspace retries: %v", err)
	}
	if err := store.RetryOf(ctx, "w-1", 1); err != nil {
		t.Fatal(err)
	}
	if dead, _ := store.DeadLettersOf(ctx, "w-1", 10); len(dead) != 1 || dead[0].ID != 2 {
		t.Fatalf("after retry = %v", dead)
	}
}
