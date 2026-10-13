// Package httpapi records every change made through the API and serves the audit log.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/audit/application"
	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// Owners finds the project of resources addressed by id alone (/sprints/{id}, /boards/{id}, /links/{id}).
type Owners interface {
	ProjectOf(ctx context.Context, kind, id string) string // "" when unknown
}

type Handler struct {
	svc    *application.Service
	owners Owners
	log    *slog.Logger
}

func New(svc *application.Service, owners Owners, log *slog.Logger) *Handler {
	return &Handler{svc: svc, owners: owners, log: log}
}

// owned reports the kind and id of /api/v1/{sprints|boards|links}/{id}/... paths.
func owned(path string) (kind, id string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) >= 2 && (parts[0] == "sprints" || parts[0] == "boards" || parts[0] == "links") && parts[1] != "" {
		return parts[0], parts[1]
	}
	return "", ""
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/audit", h.list)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Record wraps the authenticated API: every write (POST, PUT, PATCH, DELETE) by a signed-in user is
// appended to the audit log with its operation, path, project and result — refused attempts too.
// Request bodies are not stored.
func (h *Handler) Record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		// Resolve before the handler runs: a deleted board has no project afterwards.
		var owner string
		if kind, id := owned(r.URL.Path); kind != "" {
			owner = h.owners.ProjectOf(r.Context(), kind, id)
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		actor := auth.Actor(r.Context())
		if actor == "" || r.Pattern == "" { // unknown routes are not actions
			return
		}
		project := domain.ProjectOf(r.PathValue("key"), r.PathValue("issueKey"))
		if project == "" {
			project = owner
		}
		if err := h.svc.Record(r.Context(), actor, r.Pattern, r.URL.Path, project, sw.status); err != nil {
			h.log.ErrorContext(r.Context(), "audit: could not record", "action", r.Pattern, "err", err)
		}
	})
}

type entryDTO struct {
	At        time.Time `json:"at"`
	ActorID   string    `json:"actor_id"`
	ActorName string    `json:"actor_name"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Status    int       `json:"status"`
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	lines, err := h.svc.Log(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), limit)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]entryDTO, 0, len(lines))
	for _, l := range lines {
		out = append(out, entryDTO{At: l.At, ActorID: l.Actor, ActorName: l.ActorName, Action: l.Action, Target: l.Target, Status: l.Status})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}
