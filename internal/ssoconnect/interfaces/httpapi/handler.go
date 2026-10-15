// Package httpapi serves SSO connections to site admins (ADR-0018). Client secrets are
// write-only: no response ever contains one.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/ssoconnect/application"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/sso/connections", h.list)
	mux.HandleFunc("POST /api/v1/admin/sso/connections", h.create)
	mux.HandleFunc("PATCH /api/v1/admin/sso/connections/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/admin/sso/connections/{id}", h.delete)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeAdminForbidden, true
	case errors.Is(err, domain.ErrInvalidConnection):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrConnectionNotFound):
		return httpx.CodeConnectionNotFound, true
	case errors.Is(err, domain.ErrNoEncryptionKey):
		return httpx.CodeNoEncryptionKey, true
	}
	return "", false
}

type connectionDTO struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Issuer      string    `json:"issuer"`
	ClientID    string    `json:"client_id"`
	GroupsClaim string    `json:"groups_claim"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

func out(c domain.Connection) connectionDTO {
	return connectionDTO{ID: string(c.ID), Kind: string(c.Kind), Name: c.Name, Issuer: c.Issuer, ClientID: c.ClientID,
		GroupsClaim: c.GroupsClaim, Enabled: c.Enabled, CreatedAt: c.CreatedAt}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]connectionDTO, 0, len(list))
	for _, c := range list {
		items = append(items, out(c))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string `json:"name"`
		Issuer       string `json:"issuer"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GroupsClaim  string `json:"groups_claim"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	c, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), domain.Settings{Name: in.Name, Issuer: in.Issuer,
		ClientID: in.ClientID, ClientSecret: in.ClientSecret, GroupsClaim: in.GroupsClaim})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(*c))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	c, err := h.svc.Update(r.Context(), auth.Actor(r.Context()), domain.ConnectionID(r.PathValue("id")),
		application.Change{Name: in.Name, Enabled: in.Enabled})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(*c))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), domain.ConnectionID(r.PathValue("id"))); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
