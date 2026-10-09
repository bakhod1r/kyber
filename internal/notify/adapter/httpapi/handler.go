// Package httpapi exposes the caller's notifications over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/app"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notifications", h.list)
	mux.HandleFunc("POST /api/v1/notifications/read-all", h.readAll)
	mux.HandleFunc("POST /api/v1/notifications/{id}/read", h.read)
}

type notificationDTO struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	IssueKey   string    `json:"issue_key"`
	IssueTitle string    `json:"issue_title"`
	ActorName  string    `json:"actor_name"`
	Excerpt    string    `json:"excerpt"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

func codeFor(err error) (string, bool) {
	if errors.Is(err, domain.ErrNotificationNotFound) {
		return httpx.CodeNotificationNotFound, true
	}
	return "", false
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, unread, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.URL.Query().Get("unread") == "true")
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]notificationDTO, 0, len(items))
	for _, n := range items {
		out = append(out, notificationDTO{ID: string(n.ID), Kind: string(n.Kind), IssueKey: n.IssueKey, IssueTitle: n.IssueTitle,
			ActorName: n.ActorName, Excerpt: n.Excerpt, Read: n.Read(), CreatedAt: n.CreatedAt.UTC()})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out, "unread": unread})
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkRead(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkAllRead(r.Context(), auth.Actor(r.Context())); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
