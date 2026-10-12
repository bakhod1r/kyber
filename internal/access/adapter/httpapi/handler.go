// Package httpapi serves a project's permission scheme (ADR-0003): read, replace, reset.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/access/app"
	"github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/permissions", h.get)
	mux.HandleFunc("PUT /api/v1/projects/{key}/permissions", h.put)
	mux.HandleFunc("DELETE /api/v1/projects/{key}/permissions", h.reset)
}

type grantDTO struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type permissionDTO struct {
	Permission string     `json:"permission"`
	Grants     []grantDTO `json:"grants"`
}

type schemeDTO struct {
	Custom      bool            `json:"custom"`
	Permissions []permissionDTO `json:"permissions"`
}

func toDTO(t domain.GrantTable, custom bool) schemeDTO {
	out := schemeDTO{Custom: custom, Permissions: []permissionDTO{}}
	for _, p := range domain.Permissions() {
		gs := []grantDTO{}
		for _, g := range t[p] {
			gs = append(gs, grantDTO{Kind: g.Kind(), Value: g.Value()})
		}
		out.Permissions = append(out.Permissions, permissionDTO{Permission: string(p), Grants: gs})
	}
	return out
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, app.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, app.ErrInvalidScheme):
		return httpx.CodeValidation, true
	}
	return "", false
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	t, custom, err := h.svc.ProjectScheme(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(t, custom))
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Permissions []permissionDTO `json:"permissions"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	t := domain.GrantTable{}
	for _, p := range body.Permissions {
		perm := domain.Permission(p.Permission)
		t[perm] = []domain.Grant{}
		for _, g := range p.Grants {
			grant, err := domain.ParseGrant(g.Kind, g.Value)
			if err != nil {
				httpx.Error(w, r, h.log, err, codeFor)
				return
			}
			t[perm] = append(t[perm], grant)
		}
	}
	saved, err := h.svc.SetProjectScheme(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), t)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(saved, true))
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ResetProjectScheme(r.Context(), auth.Actor(r.Context()), r.PathValue("key")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
