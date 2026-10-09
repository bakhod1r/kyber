// Package httpapi exposes the Project context over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/project/app"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects", h.create)
	mux.HandleFunc("GET /api/v1/projects", h.list)
	mux.HandleFunc("GET /api/v1/projects/{key}", h.get)
	mux.HandleFunc("GET /api/v1/projects/{key}/members", h.members)
	mux.HandleFunc("POST /api/v1/projects/{key}/members", h.setMember)
}

type projectDTO struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func toDTO(p *domain.Project) projectDTO {
	return projectDTO{ID: string(p.ID()), Key: p.Key(), Name: p.Name()}
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidKey), errors.Is(err, domain.ErrEmptyName), errors.Is(err, domain.ErrInvalidRole):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrKeyTaken):
		return httpx.CodeProjectKeyTaken, true
	case errors.Is(err, domain.ErrLastAdmin):
		return httpx.CodeLastAdmin, true
	case errors.Is(err, domain.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, app.ErrUnknownUser):
		return httpx.CodeUserNotFound, true
	case errors.Is(err, domain.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key, Name string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	p, err := h.svc.Create(r.Context(), actor(r), in.Key, in.Name)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), actor(r), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(p))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.List(r.Context(), actor(r))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]projectDTO, 0, len(ps))
	for _, p := range ps {
		items = append(items, toDTO(p))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func actor(r *http.Request) domain.UserID { return domain.UserID(auth.Actor(r.Context())) }

type memberDTO struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	ms, err := h.svc.Members(r.Context(), actor(r), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]memberDTO, 0, len(ms))
	for _, m := range ms {
		items = append(items, memberDTO{UserID: string(m.ID), Email: m.Email, Name: m.Name, Role: string(m.Role)})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) setMember(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Role string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if err := h.svc.SetMember(r.Context(), actor(r), r.PathValue("key"), in.Email, in.Role); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
