package realtime

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Register serves GET /api/v1/stream (the caller's events) and GET /api/v1/presence.
func (h *Hub) Register(mux *http.ServeMux, heartbeat time.Duration) {
	mux.HandleFunc("GET /api/v1/stream", func(w http.ResponseWriter, r *http.Request) { h.stream(w, r, heartbeat) })
	mux.HandleFunc("GET /api/v1/presence", h.presence)
}

func (h *Hub) stream(w http.ResponseWriter, r *http.Request, heartbeat time.Duration) {
	last, _ := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64)
	events, stop := h.Subscribe(tenant.Workspace(r.Context()), auth.Actor(r.Context()), last)
	defer stop()
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
