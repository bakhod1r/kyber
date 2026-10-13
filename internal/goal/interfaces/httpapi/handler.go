// Package httpapi serves a project's goals.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/goal/application"
	"github.com/bakhod1r/kyber/internal/goal/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/goals", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/goals", h.create)
	mux.HandleFunc("PUT /api/v1/goals/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/goals/{id}", h.delete)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrInvalidQuery):
		return httpx.CodeInvalidJQL, true
	case errors.Is(err, domain.ErrInvalidGoal), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrGoalNotFound):
		return httpx.CodeGoalNotFound, true
	case errors.Is(err, domain.ErrGoalConflict):
		return httpx.CodeGoalConflict, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type goalDTO struct {
	ID         string  `json:"id"`
	ProjectKey string  `json:"project_key"`
	OwnerID    string  `json:"owner_id"`
	Name       string  `json:"name"`
	TargetDate *string `json:"target_date"`
	JQL        string  `json:"jql"`
	Status     string  `json:"status"`
	Done       int     `json:"done"`
	Total      int     `json:"total"`
	Percent    int     `json:"percent"`
	Version    int     `json:"version"`
}

func out(v application.View) goalDTO {
	g := v.Goal
	d := goalDTO{ID: string(g.ID), ProjectKey: g.Project, OwnerID: g.Owner, Name: g.Name, JQL: g.JQL, Status: string(g.Status),
		Done: v.Progress.Done, Total: v.Progress.Total, Percent: v.Progress.Percent(), Version: g.Version}
	if g.Target != "" {
		t := g.Target
		d.TargetDate = &t
	}
	return d
}

type goalIn struct {
	Name       string `json:"name"`
	TargetDate string `json:"target_date"`
	JQL        string `json:"jql"`
	Status     string `json:"status"`
	Version    int    `json:"version"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Goals(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]goalDTO, 0, len(list))
	for _, v := range list {
		items = append(items, out(v))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in goalIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	v, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.Name, in.TargetDate, in.JQL)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(v))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var in goalIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	v, err := h.svc.Update(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), application.Change{
		Version: in.Version, Name: in.Name, Target: in.TargetDate, JQL: in.JQL, Status: domain.Status(in.Status)})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(v))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
