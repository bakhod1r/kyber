package presence_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/presence"
	pt "github.com/bakhod1r/kyber/internal/platform/presence/presencetest"
)

type seats struct {
	mu sync.Mutex
	s  []presence.Seat
}

func (l *seats) set(s ...presence.Seat) { l.mu.Lock(); l.s = s; l.mu.Unlock() }
func (l *seats) get() []presence.Seat   { l.mu.Lock(); defer l.mu.Unlock(); return slices.Clone(l.s) }

func TestRosterAcrossReplicas(t *testing.T) {
	ctx := context.Background()
	store := presence.NewMemory()
	now := time.Now
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var a, b seats
	ra := presence.NewRoster(store, "a", a.get, now, log)
	rb := presence.NewRoster(store, "b", b.get, now, log)

	a.set(presence.Seat{Workspace: pt.WS1, User: pt.Ann})
	// Replica A's own seat counts before any beat; B learns it from the store after A beats.
	if got, _ := ra.Online(ctx, pt.WS1, []string{pt.Ann, pt.Bob}); !slices.Equal(got, []string{pt.Ann}) {
		t.Fatalf("local = %v", got)
	}
	if got, _ := rb.Online(ctx, pt.WS1, []string{pt.Ann}); len(got) != 0 {
		t.Fatalf("before the beat = %v", got)
	}
	if err := ra.Beat(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := rb.Online(ctx, pt.WS1, []string{pt.Ann}); !slices.Equal(got, []string{pt.Ann}) {
		t.Fatalf("across replicas = %v", got)
	}

	// Run beats on Changed and clears the replica when it stops.
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { ra.Run(runCtx); close(done) }()
	a.set(presence.Seat{Workspace: pt.WS1, User: pt.Bob})
	ra.Changed()
	ra.Changed() // coalesced
	waitFor(t, func() bool {
		got, _ := rb.Online(ctx, pt.WS1, []string{pt.Ann, pt.Bob})
		return slices.Equal(got, []string{pt.Bob})
	})
	stop()
	<-done
	if got, _ := rb.Online(ctx, pt.WS1, []string{pt.Bob}); len(got) != 0 {
		t.Fatalf("a stopped replica's users stay online: %v", got)
	}
}

type broken struct{ presence.Store }

func (broken) Beat(context.Context, string, []presence.Seat, time.Time) error {
	return errors.New("down")
}
func (broken) Expire(context.Context, time.Time) (int, error) { return 0, errors.New("down") }
func (broken) Online(context.Context, string, []string, time.Time) ([]string, error) {
	return nil, errors.New("down")
}

func TestRosterStoreErrors(t *testing.T) {
	var l seats
	r := presence.NewRoster(broken{}, "a", l.get, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := r.Online(context.Background(), pt.WS1, []string{pt.Ann}); err == nil {
		t.Fatal("store errors surface")
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }() // logs and keeps going
	r.Changed()
	time.Sleep(20 * time.Millisecond)
	stop()
	<-done
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out")
}
