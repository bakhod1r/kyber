package outbox_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

func TestPostgresStore(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	for i := range 50 {
		_, _ = pool.Exec(ctx, `INSERT INTO outbox (name, payload) VALUES ('e', jsonb_build_object('i', $1::int))`, i)
	}
	store := outbox.NewPostgresStore(pool)

	// Two relays (replicas) in parallel: every event exactly once.
	var handled atomic.Int64
	seen := sync.Map{}
	dup := atomic.Bool{}
	var wg sync.WaitGroup
	for range 2 {
		relay := outbox.NewRelay(store, quiet())
		relay.Handle("e", func(_ context.Context, m outbox.Message) error {
			if _, loaded := seen.LoadOrStore(m.ID, true); loaded {
				dup.Store(true)
			}
			handled.Add(1)
			return nil
		})
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, err := relay.ProcessOnce(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()
	if dup.Load() || handled.Load() != 50 {
		t.Fatalf("handled=%d duplicate=%v", handled.Load(), dup.Load())
	}
	var unpublished int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&unpublished)
	if unpublished != 0 {
		t.Fatalf("unpublished = %d", unpublished)
	}

	// A failure leaves that event and everything after it for the next pass.
	_, _ = pool.Exec(ctx, `INSERT INTO outbox (name, payload) VALUES ('f', '{}'), ('f', '{}')`)
	relay := outbox.NewRelay(store, quiet())
	calls := 0
	relay.Handle("f", func(context.Context, outbox.Message) error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	})
	if n, err := relay.ProcessOnce(ctx); err == nil || n != 0 {
		t.Fatalf("failing pass = %d, %v", n, err)
	}
	if n, err := relay.ProcessOnce(ctx); err != nil || n != 2 {
		t.Fatalf("retry pass = %d, %v", n, err)
	}
}

func TestPostgresStoreEnvelopeAndDeadLetters(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	parent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if _, err := pool.Exec(ctx, `INSERT INTO outbox (name, payload, actor_id, trace_parent, schema_version)
		VALUES ('poison', '{}', 'u-1', $1, 1), ('ok', '{}', 'u-2', '', 1)`, parent); err != nil {
		t.Fatal(err)
	}
	store := outbox.NewPostgresStore(pool)
	relay := outbox.NewRelay(store, quiet(), outbox.MaxAttempts(2))
	var got []outbox.Message
	relay.Handle("poison", func(_ context.Context, m outbox.Message) error { got = append(got, m); return errors.New("bug") })
	relay.Handle("ok", func(_ context.Context, m outbox.Message) error { got = append(got, m); return nil })

	if n, err := relay.ProcessOnce(ctx); err == nil || n != 0 || len(got) != 1 {
		t.Fatalf("first pass: n=%d err=%v got=%d", n, err, len(got))
	}
	if m := got[0]; m.Actor != "u-1" || m.Trace != parent || m.Version != 1 || m.Attempts != 0 || m.Workspace == "" {
		t.Fatalf("message = %+v", m)
	}
	if n, err := relay.ProcessOnce(ctx); err != nil || n != 1 || len(got) != 3 || got[1].Attempts != 1 || got[2].Name != "ok" {
		t.Fatalf("second pass: n=%d err=%v got=%+v", n, err, got)
	}
	dead, err := store.DeadLetters(ctx, 10)
	if err != nil || len(dead) != 1 || dead[0].Name != "poison" || dead[0].Attempts != 2 || dead[0].LastError == "" {
		t.Fatalf("dead letters = %+v %v", dead, err)
	}
	if n, _ := relay.ProcessOnce(ctx); n != 0 || len(got) != 3 {
		t.Fatalf("dead letter redelivered: n=%d", n)
	}
	if err := store.Retry(ctx, dead[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Retry(ctx, dead[0].ID); !errors.Is(err, outbox.ErrNotDead) {
		t.Fatalf("retry twice: %v", err)
	}
	fixed := outbox.NewRelay(store, quiet())
	fixed.Handle("poison", func(_ context.Context, m outbox.Message) error {
		if m.Attempts != 0 || m.LastError != "" {
			t.Errorf("retried message = %+v", m)
		}
		return nil
	})
	if n, err := fixed.ProcessOnce(ctx); err != nil || n != 1 {
		t.Fatalf("after retry: n=%d err=%v", n, err)
	}
	if dead, _ := store.DeadLetters(ctx, 10); len(dead) != 0 {
		t.Fatalf("still dead: %+v", dead)
	}
}
