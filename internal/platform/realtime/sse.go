package realtime

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Extra is an event of another feed sent on the stream (no id: it is not replayed).
type Extra struct {
	Type string
	Data []byte
}

// Feed opens another source of events for a stream (S83: board hints), closed when ctx ends.
type Feed func(ctx context.Context, ws, user string) <-chan Extra

// Register serves GET /api/v1/stream (the caller's events and the feeds') and GET /api/v1/presence.
func (h *Hub) Register(mux *http.ServeMux, heartbeat time.Duration, feeds ...Feed) {
	mux.HandleFunc("GET /api/v1/stream", func(w http.ResponseWriter, r *http.Request) { h.stream(w, r, heartbeat, feeds) })
	mux.HandleFunc("GET /api/v1/presence", h.presence)
}

func (h *Hub) stream(w http.ResponseWriter, r *http.Request, heartbeat time.Duration, feeds []Feed) {
	last, _ := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64)
	ws, user := tenant.Workspace(r.Context()), auth.Actor(r.Context())
	events, stop := h.Subscribe(ws, user, last)
	defer stop()
	extra := make(chan Extra, 16)
	for _, f := range feeds {
		go func(src <-chan Extra) {
			for e := range src {
				select {
				case extra <- e:
				case <-r.Context().Done():
					return
				}
			}
		}(f(r.Context(), ws, user))
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // a stream outlives the server's write timeout
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")
	if rc.Flush() != nil {
		return
	}
	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
		case e, ok := <-events:
			if !ok {
				return // too slow: the browser reconnects and replays
			}
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Type, e.Data)
		case e := <-extra:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, e.Data)
		}
		if rc.Flush() != nil {
			return
		}
	}
}

// presence: ?user_id=a&user_id=b → which of them have Kyber open in this workspace.
func (h *Hub) presence(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"online": h.Online(tenant.Workspace(r.Context()), r.URL.Query()["user_id"])})
}
