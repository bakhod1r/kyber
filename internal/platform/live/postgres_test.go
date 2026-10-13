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

// A change broadcast on one replica reaches the hub of another through NOTIFY.
func TestPostgresCrossesReplicas(t *testing.T) {
	pool := dbtest.New(t)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := live.NewPostgres(pool, live.NewHub(), quiet)
	hub := live.NewHub()
	receiver := live.NewPostgres(pool, hub, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { receiver.Listen(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	sub, stop := live.Subscribe(hub, "w-1")
	defer stop()
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
	l := live.NewPostgres(pool, live.NewHub(), quiet)
	live.SetRetry(l, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	l.Listen(ctx) // returns once ctx ends
	if err := l.Broadcast(context.Background(), live.Change{}); err == nil {
		t.Fatal("broadcast on a closed pool succeeded")
	}
}
