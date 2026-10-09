// Package httpapi exposes workspaces over REST and resolves the request's workspace from
// its host (ADR-0004).
package httpapi

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/workspace/app"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

type Handler struct {
	svc        *app.Service
	log        *slog.Logger
	baseDomain string // "" = single-tenant
	scheme     string
}

// New: baseDomain like "kyber.example.com" enables subdomains; "" keeps single-tenant mode.
func New(svc *app.Service, log *slog.Logger, baseDomain string, secure bool) *Handler {
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return &Handler{svc: svc, log: log, baseDomain: strings.ToLower(strings.TrimSpace(baseDomain)), scheme: scheme}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/workspaces", h.mine)
	mux.HandleFunc("POST /api/v1/workspaces", h.create)
}

var errNoWorkspace = errors.New("this address does not belong to a workspace")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrWorkspaceNotFound), errors.Is(err, errNoWorkspace):
		return httpx.CodeWorkspaceNotFound, true
	case errors.Is(err, domain.ErrSlugTaken):
		return httpx.CodeWorkspaceSlugTaken, true
	case errors.Is(err, domain.ErrInvalidSlug), errors.Is(err, domain.ErrEmptyName):
		return httpx.CodeValidation, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

type workspaceDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Role string `json:"role"`
	URL  string `json:"url"`
}

func (h *Handler) url(slug string) string {
	if h.baseDomain == "" {
		return "/"
	}
	return h.scheme + "://" + slug + "." + h.baseDomain + "/"
}

func (h *Handler) dto(w *domain.Workspace, actor string) workspaceDTO {
	role, _ := w.RoleOf(domain.UserID(actor))
	return workspaceDTO{ID: string(w.ID()), Slug: w.Slug(), Name: w.Name(), Role: string(role), URL: h.url(w.Slug())}
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	actor := auth.Actor(r.Context())
	list, err := h.svc.Mine(r.Context(), actor)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]workspaceDTO, 0, len(list))
	for _, ws := range list {
		out = append(out, h.dto(ws, actor))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Slug, Name string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	ws, err := h.svc.Create(r.Context(), actor, in.Slug, in.Name)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, h.dto(ws, actor))
}

// apexAPI are the API paths served on the apex domain in multi-tenant mode
// (entries ending in "/" are prefixes, the others exact paths).
var apexAPI = []string{"/api/v1/auth/", "/api/v1/me", "/api/v1/workspaces"}

// probes answer on any host (load balancers and Prometheus use IPs).
var probes = []string{"/healthz", "/readyz", "/metrics"}

func hasPrefix(path string, list []string) bool {
	for _, p := range list {
		if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}

// Resolve scopes every request to a workspace: the default one in single-tenant mode,
// otherwise the one named by the subdomain. Unknown hosts get 404 (no Host-header games).
func (h *Handler) Resolve(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.baseDomain == "" {
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), string(domain.DefaultID))))
			return
		}
		if hasPrefix(r.URL.Path, probes) {
			next.ServeHTTP(w, r)
			return
		}
		host := strings.ToLower(r.Host)
		if hp, _, err := net.SplitHostPort(host); err == nil {
			host = hp
		}
		if host == h.baseDomain {
			if strings.HasPrefix(r.URL.Path, "/api/") && !hasPrefix(r.URL.Path, apexAPI) {
				httpx.Error(w, r, h.log, errNoWorkspace, codeFor)
				return
			}
			next.ServeHTTP(w, r) // apex: landing, sign-up, login, workspace picker
			return
		}
		slug, ok := strings.CutSuffix(host, "."+h.baseDomain)
		if !ok || strings.Contains(slug, ".") {
			httpx.Error(w, r, h.log, errNoWorkspace, codeFor)
			return
		}
		ws, err := h.svc.Resolve(r.Context(), slug)
		if err != nil {
			httpx.Error(w, r, h.log, err, codeFor)
			return
		}
		next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), string(ws.ID()))))
	})
}

// RequireMember (inside authentication) hides a workspace's API from non-members.
func (h *Handler) RequireMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, scoped := tenant.From(r.Context())
		if scoped && !hasPrefix(r.URL.Path, apexAPI) {
			ok, err := h.svc.IsMember(r.Context(), ws, auth.Actor(r.Context()))
			if err != nil || !ok {
				httpx.Error(w, r, h.log, errors.Join(errNoWorkspace, err), codeFor)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
