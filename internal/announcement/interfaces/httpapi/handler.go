// Package httpapi serves announcements of a project and of the workspace.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/application"
	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/announcements", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/announcements", h.publish)
	mux.HandleFunc("GET /api/v1/announcements", h.list)
	mux.HandleFunc("POST /api/v1/announcements", h.publish)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidAnnouncement), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type announcementDTO struct {
	ID         string    `json:"id"`
	ProjectKey *string   `json:"project_key"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	AuthorID   string    `json:"author_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func out(a *domain.Announcement) announcementDTO {
	d := announcementDTO{ID: string(a.ID), Title: a.Title, Body: a.Body, AuthorID: a.Author, CreatedAt: a.At}
	if a.Project != "" {
		p := a.Project
		d.ProjectKey = &p
	}
	return d
}

// The project routes carry {key}; the workspace routes have none ("").
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]announcementDTO, 0, len(list))
	for _, a := range list {
		items = append(items, out(a))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	var in struct{ Title, Body string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	a, err := h.svc.Publish(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.Title, in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(a))
}
