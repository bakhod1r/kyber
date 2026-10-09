// Package httpapi exposes the Project context over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

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
}

type projectDTO struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func toDTO(p *domain.Project) projectDTO {
	return projectDTO{ID: string(p.ID()), Key: p.Key(), Name: p.Name()}
}

func statusFor(err error) (int, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidKey), errors.Is(err, domain.ErrEmptyName):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, domain.ErrKeyTaken):
		return http.StatusConflict, true
	case errors.Is(err, domain.ErrProjectNotFound):
		return http.StatusNotFound, true
	}
	return 0, false
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key, Name string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	p, err := h.svc.Create(r.Context(), in.Key, in.Name)
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(p))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.List(r.Context())
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	items := make([]projectDTO, 0, len(ps))
	for _, p := range ps {
		items = append(items, toDTO(p))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
