// Package httpapi serves JQL search and saved filters.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/search/application"
	"github.com/bakhod1r/kyber/internal/search/domain"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/search", h.search)
	mux.HandleFunc("GET /api/v1/filters", h.filters)
	mux.HandleFunc("POST /api/v1/filters", h.createFilter)
	mux.HandleFunc("PUT /api/v1/filters/{id}", h.updateFilter)
	mux.HandleFunc("DELETE /api/v1/filters/{id}", h.deleteFilter)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidJQL):
		return httpx.CodeInvalidJQL, true
	case errors.Is(err, domain.ErrInvalidFilter), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrFilterNotFound):
		return httpx.CodeFilterNotFound, true
	case errors.Is(err, domain.ErrFilterConflict):
		return httpx.CodeFilterConflict, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type issueDTO struct {
	Key            string   `json:"key"`
	ProjectKey     string   `json:"project_key"`
	Title          string   `json:"title"`
	Type           string   `json:"type"`
	Status         string   `json:"status"`
	StatusName     string   `json:"status_name"`
	StatusCategory string   `json:"status_category"`
	Priority       string   `json:"priority"`
	AssigneeID     *string  `json:"assignee_id"`
	Resolution     string   `json:"resolution"`
	Points         *float64 `json:"points"`
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	res, err := h.svc.Search(r.Context(), auth.Actor(r.Context()), r.URL.Query().Get("jql"), limit)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]issueDTO, 0, len(res.Issues))
	for _, is := range res.Issues {
		d := issueDTO{Key: is.Key, ProjectKey: is.Project, Title: is.Summary, Type: is.Type, Status: is.Status, StatusName: is.StatusName,
			StatusCategory: is.Category, Priority: is.Priority, Resolution: is.Resolution, Points: is.Points}
		if is.Assignee != "" {
			a := is.Assignee
			d.AssigneeID = &a
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"total": res.Total, "items": out})
}

type filterDTO struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
	Name    string `json:"name"`
	JQL     string `json:"jql"`
	Shared  bool   `json:"shared"`
	Version int    `json:"version"`
	Mine    bool   `json:"mine"`
}

func filterOut(f *domain.Filter, actor string) filterDTO {
	return filterDTO{ID: string(f.ID), OwnerID: f.Owner, Name: f.Name, JQL: f.JQL, Shared: f.Shared, Version: f.Version, Mine: f.Owner == actor}
}

type filterIn struct {
	Name    string `json:"name"`
	JQL     string `json:"jql"`
	Shared  bool   `json:"shared"`
	Version int    `json:"version"`
}

func (h *Handler) filters(w http.ResponseWriter, r *http.Request) {
	actor := auth.Actor(r.Context())
	list, err := h.svc.Filters(r.Context(), actor)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]filterDTO, 0, len(list))
	for _, f := range list {
		out = append(out, filterOut(f, actor))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createFilter(w http.ResponseWriter, r *http.Request) {
	var in filterIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	f, err := h.svc.SaveFilter(r.Context(), actor, in.Name, in.JQL, in.Shared)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, filterOut(f, actor))
}

func (h *Handler) updateFilter(w http.ResponseWriter, r *http.Request) {
	var in filterIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	f, err := h.svc.UpdateFilter(r.Context(), actor, r.PathValue("id"), in.Version, in.Name, in.JQL, in.Shared)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, filterOut(f, actor))
}

func (h *Handler) deleteFilter(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteFilter(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
