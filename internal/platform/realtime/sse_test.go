package realtime_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/realtime"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// S83: other streams (board hints) ride on the same SSE connection as chat events.
func TestStreamCarriesFeeds(t *testing.T) {
	h := realtime.NewHub(16)
	feed := func(ctx context.Context, ws, user string) <-chan realtime.Extra {
		out := make(chan realtime.Extra, 1)
		out <- realtime.Extra{Type: "change", Data: []byte(`{"project":"KYB","ws":"` + ws + `","user":"` + user + `"}`)}
		return out
	}
	mux := http.NewServeMux()
	h.Register(mux, time.Minute, feed)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.WithActor(tenant.With(r.Context(), "ws-1"), "u-1")
		mux.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/v1/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	deadline := time.After(3 * time.Second)
	lines := make(chan string)
	go func() {
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	for {
		select {
		case l := <-lines:
			if strings.HasPrefix(l, "data: ") && strings.Contains(l, `"project":"KYB","ws":"ws-1","user":"u-1"`) {
				return
			}
		case <-deadline:
			t.Fatal("no feed event on the stream")
		}
	}
}
