package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// PresenceHandler exposes who is viewing an issue right now.
type PresenceHandler struct {
	svc *application.Presence
	log *slog.Logger
}

func NewPresence(svc *application.Presence, log *slog.Logger) *PresenceHandler {
	return &PresenceHandler{svc: svc, log: log}
}

func (h *PresenceHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/presence", h.heartbeat)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/presence", h.viewers)
}

func (h *PresenceHandler) heartbeat(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Heartbeat(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PresenceHandler) viewers(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Viewers(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]watcherDTO, 0, len(list))
	for _, v := range list {
		out = append(out, watcherDTO{UserID: v.ID, Name: v.Name})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}
