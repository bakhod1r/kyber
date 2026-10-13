// Package httpapi serves the workspace administration panel.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/admin/application"
	"github.com/bakhod1r/kyber/internal/admin/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/access", h.access)
	mux.HandleFunc("GET /api/v1/admin/members", h.members)
	mux.HandleFunc("PUT /api/v1/admin/members/{id}", h.changeRole)
	mux.HandleFunc("GET /api/v1/admin/projects", h.projects)
	mux.HandleFunc("GET /api/v1/admin/dead-letters", h.deadLetters)
	mux.HandleFunc("POST /api/v1/admin/dead-letters/{id}/retry", h.retry)
}

var errBadID = errors.New("dead letter ids are numbers")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return httpx.CodeAdminForbidden, true
	case errors.Is(err, domain.ErrSingleTenant):
		return httpx.CodeSingleTenantRoles, true
	case errors.Is(err, domain.ErrLastOwner):
		return httpx.CodeLastOwner, true
	case errors.Is(err, domain.ErrNotMember):
		return httpx.CodeMemberNotFound, true
	case errors.Is(err, domain.ErrNoDeadLetter), errors.Is(err, errBadID):
		return httpx.CodeDeadLetterNotFound, true
	case errors.Is(err, domain.ErrInvalidRole), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	}
	return "", false
}

func (h *Handler) access(w http.ResponseWriter, r *http.Request) {
	ok, err := h.svc.Access(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"admin": ok})
}

type memberDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	ms, err := h.svc.Members(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]memberDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, memberDTO(m))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if err := h.svc.ChangeRole(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Role); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type projectDTO struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Members int    `json:"members"`
}

func (h *Handler) projects(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.Projects(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]projectDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, projectDTO(p))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type deadLetterDTO struct {
	ID        int64     `json:"id"`
	Event     string    `json:"event"`
	ActorID   string    `json:"actor_id"`
	At        time.Time `json:"at"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error"`
}

func (h *Handler) deadLetters(w http.ResponseWriter, r *http.Request) {
	dl, err := h.svc.DeadLetters(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]deadLetterDTO, 0, len(dl))
	for _, d := range dl {
		out = append(out, deadLetterDTO{ID: d.ID, Event: d.Event, ActorID: d.Actor, At: d.At.UTC(), Attempts: d.Attempts, LastError: d.LastError})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		err = errBadID
	} else {
		err = h.svc.Retry(r.Context(), auth.Actor(r.Context()), id)
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
