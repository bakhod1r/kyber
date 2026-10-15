// Package httpapi serves SSO connections to site admins (ADR-0018). Client secrets are
// write-only: no response ever contains one.
package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/bakhod1r/errorx"

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
	mux.HandleFunc("POST /api/v1/admin/sso/connections/{id}/domains", h.addDomain)
	mux.HandleFunc("POST /api/v1/admin/sso/connections/{id}/domains/{domain}/verify", h.verifyDomain)
	mux.HandleFunc("DELETE /api/v1/admin/sso/connections/{id}/domains/{domain}", h.removeDomain)
}

// RegisterPublic serves email-first discovery on the login page (no session yet).
func (h *Handler) RegisterPublic(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/auth/sso/discover", wrap(http.HandlerFunc(h.discover)))
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
	case errors.Is(err, domain.ErrDomainNotFound):
		return httpx.CodeSSODomainNotFound, true
	case errors.Is(err, domain.ErrDomainNotVerified):
		return httpx.CodeSSODomainNotVerified, true
	case errors.Is(err, domain.ErrDomainTaken):
		return httpx.CodeSSODomainTaken, true
	}
	return "", false
}

type connectionDTO struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Issuer      string      `json:"issuer"`
	ClientID    string      `json:"client_id"`
	GroupsClaim string      `json:"groups_claim"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	Domains     []domainDTO `json:"domains"`
}

// domainDTO shows the TXT record to publish; the token is not a secret (it only proves DNS control).
type domainDTO struct {
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
	TXT      string `json:"txt"`
}

func out(c domain.Connection) connectionDTO {
	ds := make([]domainDTO, 0, len(c.Domains))
	for _, d := range c.Domains {
		ds = append(ds, domainDTO{Name: d.Name, Verified: d.Verified, TXT: "kyber-verify=" + d.Token})
	}
	return connectionDTO{ID: string(c.ID), Kind: string(c.Kind), Name: c.Name, Issuer: c.Issuer, ClientID: c.ClientID,
		GroupsClaim: c.GroupsClaim, Enabled: c.Enabled, CreatedAt: c.CreatedAt, Domains: ds}
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

func (h *Handler) reply(w http.ResponseWriter, r *http.Request, c *domain.Connection, err error) {
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(*c))
}

func (h *Handler) addDomain(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domain  string `json:"domain"`
		Trusted bool   `json:"trusted"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	c, err := h.svc.AddDomain(r.Context(), auth.Actor(r.Context()), domain.ConnectionID(r.PathValue("id")), in.Domain, in.Trusted)
	h.reply(w, r, c, err)
}

func (h *Handler) verifyDomain(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.VerifyDomain(r.Context(), auth.Actor(r.Context()), domain.ConnectionID(r.PathValue("id")), r.PathValue("domain"))
	h.reply(w, r, c, err)
}

func (h *Handler) removeDomain(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.RemoveDomain(r.Context(), auth.Actor(r.Context()), domain.ConnectionID(r.PathValue("id")), r.PathValue("domain"))
	h.reply(w, r, c, err)
}

// discover answers which connection signs in this email, or null. JSON only, so a cross-site
// form cannot post here without a CORS preflight.
func (h *Handler) discover(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		msg := "sign-in requests require Content-Type: application/json"
		httpx.Problem(w, r, errorx.New(httpx.CodeCSRF, msg).WithDetails(msg))
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	type button struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var found *button
	c, err := h.svc.Discover(r.Context(), in.Email)
	switch {
	case err == nil:
		found = &button{ID: string(c.ID), Name: c.Name}
	case !errors.Is(err, domain.ErrConnectionNotFound):
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"connection": found})
}
