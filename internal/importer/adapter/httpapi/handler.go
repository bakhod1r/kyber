// Package httpapi exposes the Importer over REST.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/importer/app"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{key}/import/jira", h.jira)
	mux.HandleFunc("GET /api/v1/projects/{key}/export.csv", h.export)
}

var errBadDryRun = errors.New("dry_run must be true or false")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, app.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, app.ErrTooLarge):
		return httpx.CodeImportTooLarge, true
	case errors.Is(err, app.ErrInvalidCSV):
		return httpx.CodeImportInvalidFile, true
	case errors.Is(err, errBadDryRun):
		return httpx.CodeValidation, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

type itemDTO struct {
	Line        int        `json:"line"`
	ExternalKey string     `json:"external_key"`
	IssueKey    *string    `json:"issue_key"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	AssigneeID  *string    `json:"assignee_id"`
	Points      *float64   `json:"points"`
	CreatedAt   *time.Time `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	Warnings    []string   `json:"warnings"`
}

type errorDTO struct {
	Line        int    `json:"line"`
	ExternalKey string `json:"external_key"`
	Message     string `json:"message"`
}

type reportDTO struct {
	DryRun  bool       `json:"dry_run"`
	Items   []itemDTO  `json:"items"`
	Errors  []errorDTO `json:"errors"`
	Skipped []string   `json:"skipped"`
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *Handler) jira(w http.ResponseWriter, r *http.Request) {
	dryRun := true
	switch r.URL.Query().Get("dry_run") {
	case "", "true":
	case "false":
		dryRun = false
	default:
		httpx.Error(w, r, h.log, errBadDryRun, codeFor)
		return
	}
	var body struct {
		CSV string `json:"csv"`
	}
	// JSON escaping can roughly double a CSV; the service enforces the real 10 MB limit.
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*app.MaxCSVBytes+1024))
	if err := dec.Decode(&body); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Error(w, r, h.log, app.ErrTooLarge, codeFor)
			return
		}
		httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
		return
	}
	rep, err := h.svc.Import(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), body.CSV, dryRun)
	if err != nil && len(rep.Items) == 0 {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if err != nil { // partial run: report what was created; re-running resumes
		h.log.ErrorContext(r.Context(), "import stopped", "project", r.PathValue("key"), "err", err)
	}
	out := reportDTO{DryRun: rep.DryRun, Items: []itemDTO{}, Errors: []errorDTO{}, Skipped: rep.Skipped}
	if out.Skipped == nil {
		out.Skipped = []string{}
	}
	for _, it := range rep.Items {
		d := itemDTO{Line: it.Line, ExternalKey: it.Key, IssueKey: optional(it.IssueKey), Title: it.Summary, Type: it.Type,
			Status: it.Status, Priority: it.Priority, AssigneeID: optional(it.AssigneeID), Points: it.Points,
			ResolvedAt: it.Resolved, Warnings: it.Warnings}
		if !it.Created.IsZero() {
			c := it.Created
			d.CreatedAt = &c
		}
		if d.Warnings == nil {
			d.Warnings = []string{}
		}
		out.Items = append(out.Items, d)
	}
	for _, e := range rep.Errors {
		out.Errors = append(out.Errors, errorDTO{Line: e.Line, ExternalKey: e.Key, Message: e.Message})
	}
	status := http.StatusOK
	if !dryRun && err == nil {
		status = http.StatusCreated
	} else if err != nil {
		status = http.StatusMultiStatus
	}
	httpx.JSON(w, status, out)
}

// export buffers the CSV so a failure still yields a proper problem response.
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var buf bytes.Buffer
	if err := h.svc.Export(r.Context(), auth.Actor(r.Context()), key, &buf); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", key+"-issues.csv"))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}
