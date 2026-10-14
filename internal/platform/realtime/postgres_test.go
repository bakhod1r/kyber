package realtime_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/realtime"
)

// Two replicas share one database: what one publishes reaches the other's streams, small events through
// NOTIFY itself and large ones (over NOTIFY's 8 kB) through a spill table.
func TestPostgresFanout(t *testing.T) {
	pool := dbtest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hubA, hubB := realtime.NewHub(8), realtime.NewHub(8)
	a := realtime.NewPostgres(pool, hubA, slog.New(slog.DiscardHandler))
	b := realtime.NewPostgres(pool, hubB, slog.New(slog.DiscardHandler))
	go a.Listen(ctx)
	go b.Listen(ctx)
	onB, stop := hubB.Subscribe("w1", "dev", 0)
	defer stop()
	onA, stopA := hubA.Subscribe("w1", "dev", 0)
	defer stopA()
	time.Sleep(300 * time.Millisecond) // both listeners are LISTENing

	big := strings.Repeat("x", 20_000)
	for _, data := range []string{"hi", big} {
		if err := a.Publish("w1", []string{"dev"}, "message.posted", map[string]string{"body": data}); err != nil {
			t.Fatal(err)
		}
		for _, ch := range []<-chan realtime.Event{onB, onA} { // the publisher's own replica gets it too, once
			e := recv(t, ch)
			if e.Type != "message.posted" || !strings.Contains(string(e.Data), data) {
				t.Fatalf("event = %s %.40s", e.Type, e.Data)
			}
		}
	}
	none(t, onA)
}
