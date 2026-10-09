// Package httpapi exposes the Issue Tracking context over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{key}/issues", h.create)
	mux.HandleFunc("GET /api/v1/projects/{key}/issues", h.list)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}", h.get)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/transitions", h.transition)
}

type issueDTO struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

func toDTO(is *domain.Issue) issueDTO {
	return issueDTO{ID: string(is.ID()), Key: is.Key().String(), Title: is.Title(),
		Type: string(is.Type()), Status: string(is.Status())}
}

func statusFor(err error) (int, bool) {
	switch {
	case errors.Is(err, app.ErrInvalidKey):
		return http.StatusBadRequest, true
	case errors.Is(err, domain.ErrEmptyTitle), errors.Is(err, domain.ErrInvalidIssueType):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, app.ErrProjectNotFound), errors.Is(err, domain.ErrIssueNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, domain.ErrTransitionNotAllowed), errors.Is(err, domain.ErrConcurrentModification):
		return http.StatusConflict, true
	}
	return 0, false
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Title, Type string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	is, err := h.svc.Create(r.Context(), app.CreateIssue{Project: r.PathValue("key"), Title: in.Title, Type: in.Type})
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(is))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	is, err := h.svc.Get(r.Context(), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	var in struct{ To string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	is, err := h.svc.Transition(r.Context(), r.PathValue("issueKey"), in.To)
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), r.PathValue("key"), r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	items := make([]issueDTO, 0, len(list))
	for _, is := range list {
		items = append(items, toDTO(is))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
