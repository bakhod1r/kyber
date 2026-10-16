// Package httpapi serves a project's automation rules and their runs.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/automation/application"
	"github.com/bakhod1r/kyber/internal/automation/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/automation/rules", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/automation/rules", h.create)
	mux.HandleFunc("GET /api/v1/projects/{key}/automation/runs", h.runs)
	mux.HandleFunc("PUT /api/v1/automation/rules/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/automation/rules/{id}", h.delete)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrInvalidCondition):
		return httpx.CodeInvalidJQL, true
	case errors.Is(err, domain.ErrInvalidRule), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrRuleNotFound):
		return httpx.CodeRuleNotFound, true
	case errors.Is(err, domain.ErrRuleConflict):
		return httpx.CodeRuleConflict, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type ruleDTO struct {
	ID         string          `json:"id"`
	ProjectKey string          `json:"project_key"`
	OwnerID    string          `json:"owner_id"`
	Name       string          `json:"name"`
	Trigger    string          `json:"trigger"`
	ToStatus   string          `json:"to_status"`
	Condition  string          `json:"condition"`
	Actions    []domain.Action `json:"actions"`
	Enabled    bool            `json:"enabled"`
	Version    int             `json:"version"`
}

func toDTO(r *domain.Rule) ruleDTO {
	return ruleDTO{ID: string(r.ID), ProjectKey: r.Project, OwnerID: r.Owner, Name: r.Name, Trigger: string(r.Trigger), ToStatus: r.ToStatus,
		Condition: r.Condition, Actions: r.Actions, Enabled: r.Enabled, Version: r.Version}
}

type ruleInput struct {
	Name      string          `json:"name"`
	Trigger   string          `json:"trigger"`
	ToStatus  string          `json:"to_status"`
	Condition string          `json:"condition"`
	Actions   []domain.Action `json:"actions"`
	Enabled   *bool           `json:"enabled"`
	Version   int             `json:"version"`
}

func (in ruleInput) spec() domain.RuleSpec {
	return domain.RuleSpec{Name: in.Name, Trigger: domain.Trigger(in.Trigger), ToStatus: in.ToStatus, Condition: in.Condition, Actions: in.Actions}
}

func actor(r *http.Request) string { return auth.Actor(r.Context()) }

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.Rules(r.Context(), actor(r), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]ruleDTO, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toDTO(rule))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in ruleInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	rule, err := h.svc.Create(r.Context(), actor(r), r.PathValue("key"), in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(rule))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var in ruleInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	rule, err := h.svc.Update(r.Context(), actor(r), r.PathValue("id"), in.Version, in.spec(), in.Enabled)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(rule))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), actor(r), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type runDTO struct {
	RuleID   string    `json:"rule_id"`
	IssueKey string    `json:"issue_key"`
	At       time.Time `json:"at"`
	OK       bool      `json:"ok"`
	Detail   string    `json:"detail"`
}

func (h *Handler) runs(w http.ResponseWriter, r *http.Request) {
	runs, err := h.svc.Runs(r.Context(), actor(r), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]runDTO, 0, len(runs))
	for _, run := range runs {
		items = append(items, runDTO{RuleID: string(run.Rule), IssueKey: run.IssueKey, At: run.At, OK: run.OK, Detail: run.Detail})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
