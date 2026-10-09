// Package httpapi exposes the Agile context over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/app"
	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/sprints", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/sprints", h.create)
	mux.HandleFunc("POST /api/v1/sprints/{id}/start", h.start)
	mux.HandleFunc("POST /api/v1/sprints/{id}/complete", h.complete)
}

type sprintDTO struct {
	ID          string     `json:"id"`
	ProjectKey  string     `json:"project_key"`
	Name        string     `json:"name"`
	Goal        string     `json:"goal"`
	State       string     `json:"state"`
	StartedAt   *time.Time `json:"started_at"`
	EndsAt      *time.Time `json:"ends_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func toDTO(s *domain.Sprint) sprintDTO {
	d := sprintDTO{ID: string(s.ID()), ProjectKey: s.Project(), Name: s.Name(), Goal: s.Goal(), State: string(s.State())}
	if t := s.StartedAt(); !t.IsZero() {
		u := t.UTC()
		d.StartedAt = &u
	}
	if t := s.EndsAt(); !t.IsZero() {
		u := t.UTC()
		d.EndsAt = &u
	}
	if t := s.CompletedAt(); !t.IsZero() {
		u := t.UTC()
		d.CompletedAt = &u
	}
	return d
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidSprint), errors.Is(err, domain.ErrInvalidSprintDates), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrSprintNotFound):
		return httpx.CodeSprintNotFound, true
	case errors.Is(err, domain.ErrAnotherSprintLive):
		return httpx.CodeSprintAlreadyActive, true
	case errors.Is(err, domain.ErrSprintConflict):
		return httpx.CodeSprintConflict, true
	case errors.Is(err, domain.ErrSprintState):
		return httpx.CodeSprintState, true
	case errors.Is(err, app.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, app.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]sprintDTO, 0, len(list))
	for _, s := range list {
		items = append(items, toDTO(s))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Goal string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	s, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.Name, in.Goal)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(s))
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	// The body is optional: {"ends_at": "<RFC 3339>"}; without it the sprint lasts 14 days.
	var in struct {
		EndsAt time.Time `json:"ends_at"`
	}
	if r.ContentLength != 0 {
		if err := httpx.Decode(r, &in); err != nil {
			httpx.Error(w, r, h.log, err, codeFor)
			return
		}
	}
	s, err := h.svc.Start(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.EndsAt)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(s))
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Complete(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sprint": toDTO(c.Sprint), "completed": c.Completed, "returned": c.Returned})
}
