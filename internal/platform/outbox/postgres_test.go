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
