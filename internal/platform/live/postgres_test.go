package live_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/live"
)

// sink records what reaches this replica.
type sink chan live.Change

func (s sink) Publish(c live.Change) { s <- c }

// A change broadcast on one replica reaches the broker of another through NOTIFY.
func TestPostgresCrossesReplicas(t *testing.T) {
	pool := dbtest.New(t)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := live.NewPostgres(pool, make(sink, 1), quiet)
	sub := make(sink, 100)
	receiver := live.NewPostgres(pool, sub, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { receiver.Listen(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	deadline := time.After(5 * time.Second)
	for {
		// LISTEN starts asynchronously: resend until the receiver hears one.
		if err := sender.Broadcast(context.Background(), live.Change{Workspace: "w-1", Project: "KYB", Event: "issue.created"}); err != nil {
			t.Fatal(err)
		}
		select {
		case c := <-sub:
			if c.Project != "KYB" || c.Event != "issue.created" || c.Workspace != "w-1" {
				t.Fatalf("change = %+v", c)
			}
			return
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatal("no notification crossed")
		}
	}
}

// The listener keeps retrying while the database is unreachable and stops with its context.
func TestPostgresListenerRetriesUntilCancelled(t *testing.T) {
	pool := dbtest.New(t)
	pool.Close() // every Acquire fails
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	l := live.NewPostgres(pool, make(sink, 1), quiet)
	live.SetRetry(l, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	l.Listen(ctx) // returns once ctx ends
	if err := l.Broadcast(context.Background(), live.Change{}); err == nil {
		t.Fatal("broadcast on a closed pool succeeded")
	}
}
