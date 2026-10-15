// Package httpapi serves SSO group mappings to site admins (S106).
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/ssosync/application"
	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/sso/group-mappings", h.list)
	mux.HandleFunc("POST /api/v1/admin/sso/group-mappings", h.create)
	mux.HandleFunc("DELETE /api/v1/admin/sso/group-mappings/{id}", h.delete)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeAdminForbidden, true
	case errors.Is(err, domain.ErrInvalidMapping):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrMappingNotFound):
		return httpx.CodeGroupMappingNotFound, true
	}
	return "", false
}

type mappingDTO struct {
	ID          string    `json:"id"`
	Group       string    `json:"group"`
	Scope       string    `json:"scope"`
	WorkspaceID string    `json:"workspace_id"`
	ProjectKey  string    `json:"project_key"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func out(m domain.Mapping) mappingDTO {
	return mappingDTO{ID: string(m.ID), Group: m.Group, Scope: string(m.Target.Scope), WorkspaceID: m.Target.Workspace,
		ProjectKey: m.Target.Project, Role: m.Role, CreatedAt: m.CreatedAt}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]mappingDTO, 0, len(list))
	for _, m := range list {
		items = append(items, out(m))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// create maps a group to a role in this workspace (or the workspace_id given).
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Group       string `json:"group"`
		Scope       string `json:"scope"`
		WorkspaceID string `json:"workspace_id"`
		ProjectKey  string `json:"project_key"`
		Role        string `json:"role"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if in.WorkspaceID == "" {
		in.WorkspaceID = tenant.Workspace(r.Context())
	}
	m, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), in.Group,
		domain.Target{Scope: domain.Scope(in.Scope), Workspace: in.WorkspaceID, Project: in.ProjectKey}, in.Role)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(*m))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), domain.MappingID(r.PathValue("id"))); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
