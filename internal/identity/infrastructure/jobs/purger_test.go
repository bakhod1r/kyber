package jobs_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/infrastructure/jobs"
)

type purger struct{ calls atomic.Int32 }

func (p *purger) PurgeExpiredSessions(context.Context) (int, error) { return int(p.calls.Add(1)), nil }

func TestRunSessionPurgerStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &purger{}
	purged := make(chan int, 100)
	done := make(chan struct{})
	go func() {
		jobs.RunSessionPurger(ctx, p, time.Millisecond, func(n int, err error) {
			if err == nil {
				purged <- n
			}
		})
		close(done)
	}()
	if n := <-purged; n != 1 {
		t.Fatalf("first purge = %d", n)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("purger did not stop")
	}
}
