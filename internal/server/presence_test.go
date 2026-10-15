package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/server"
)

// S105: two replicas on one database agree on who is online, and a closed stream goes offline on
// the other replica too.
func TestPresenceAcrossReplicas(t *testing.T) {
	pool := dbtest.New(t)
	// Each replica has its own pool, as in production (each holds LISTEN connections).
	other, err := pgxpool.NewWithConfig(context.Background(), pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Close) // after t.Context ends the replicas' LISTEN loops
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := httptest.NewServer(server.NewPostgres(log, pool, false, server.WithContext(t.Context())))
	defer a.Close()
	b := httptest.NewServer(server.NewPostgres(log, other, false, server.WithContext(t.Context())))
	defer b.Close()

	ann := (&client{t: t, srv: a}).signedIn("ann@x.uz")
	bob := &client{t: t, srv: b, token: (&client{t: t, srv: a}).signedIn("bob@x.uz").token} // asks replica B
	_, me := ann.do("GET", "/api/v1/me", nil)
	annID := me["id"].(string)
	online := func() bool {
		_, on := bob.do("GET", "/api/v1/presence?user_id="+annID, nil)
		return len(on["online"].([]any)) == 1
	}
	if online() {
		t.Fatal("online before connecting")
	}

	ctx, closeStream := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", a.URL+"/api/v1/stream", nil)
	req.Header.Set("Authorization", "Bearer "+ann.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("stream on A: %v %v", err, res)
	}
	spec.exercised(req)
	eventually(t, "online on replica B", online)

	closeStream()
	res.Body.Close()
	eventually(t, "offline on replica B after the stream closed", func() bool { return !online() })
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting: %s", what)
}
