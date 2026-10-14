// Package httpapi serves a project's webhooks (administrators), an issue's development links,
// and the GitHub integration endpoint.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/webhook/application"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

var (
	errSignature = errors.New("X-Hub-Signature-256 does not match the body")
	errDisabled  = errors.New("the GitHub integration is not configured (KYBER_GITHUB_WEBHOOK_SECRET)")
)

// maxGitHubBody bounds a GitHub delivery (GitHub caps payloads at 25 MB; pushes with many commits
// are the largest, and only their first commits matter here).
const maxGitHubBody = 5 << 20

type Handler struct {
	svc    *application.Service
	log    *slog.Logger
	github string // shared secret of GitHub webhooks; "" disables the endpoint
}

func New(svc *application.Service, log *slog.Logger, githubSecret string) *Handler {
	return &Handler{svc: svc, log: log, github: githubSecret}
}

// Register mounts the authenticated routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/webhooks", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/webhooks", h.create)
	mux.HandleFunc("PUT /api/v1/projects/{key}/webhooks/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/projects/{key}/webhooks/{id}", h.delete)
	mux.HandleFunc("GET /api/v1/projects/{key}/webhooks/{id}/deliveries", h.deliveries)
	mux.HandleFunc("GET /api/v1/issues/{key}/development", h.development)
}

// RegisterPublic mounts the GitHub endpoint, authenticated by its HMAC signature instead of a session.
func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/integrations/github", h.githubHook)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidWebhook):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrWebhookNotFound):
		return httpx.CodeWebhookNotFound, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrIssueNotFound):
		return httpx.CodeIssueNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, errSignature):
		return httpx.CodeWebhookSignature, true
	case errors.Is(err, errDisabled):
		return httpx.CodeNotFound, true
	}
	return "", false
}

type webhookDTO struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Secret    string    `json:"secret,omitempty"` // only in the response that creates the webhook
}

func out(w *domain.Webhook) webhookDTO {
	return webhookDTO{ID: string(w.ID), Project: w.Project, URL: w.URL, Events: w.Events, Active: w.Active,
		CreatedBy: w.CreatedBy, CreatedAt: w.CreatedAt}
}

type webhookInput struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Active *bool    `json:"active"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Webhooks(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]webhookDTO, 0, len(list))
	for _, wh := range list {
		items = append(items, out(wh))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in webhookInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	wh, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.URL, in.Events)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	dto := out(wh)
	dto.Secret = wh.Secret
	httpx.JSON(w, http.StatusCreated, dto)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var in webhookInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	active := in.Active == nil || *in.Active
	wh, err := h.svc.Update(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), r.PathValue("id"), in.URL, in.Events, active)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(wh))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type deliveryDTO struct {
	ID          string     `json:"id"`
	Event       string     `json:"event"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastStatus  int        `json:"last_status"`
	LastError   string     `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
	NextAt      *time.Time `json:"next_at,omitempty"` // pending only
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

func (h *Handler) deliveries(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Deliveries(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]deliveryDTO, 0, len(list))
	for _, d := range list {
		dto := deliveryDTO{ID: d.ID, Event: d.Event, Status: string(d.Status), Attempts: d.Attempts, LastStatus: d.LastStatus,
			LastError: d.LastError, CreatedAt: d.CreatedAt}
		if d.Status == domain.DeliveryPending {
			dto.NextAt = &d.NextAt
		}
		if !d.DeliveredAt.IsZero() {
			dto.DeliveredAt = &d.DeliveredAt
		}
		items = append(items, dto)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type linkDTO struct {
	Kind      string    `json:"kind"`
	Repo      string    `json:"repo"`
	Ref       string    `json:"ref"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *Handler) development(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Development(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]linkDTO, 0, len(list))
	for _, l := range list {
		items = append(items, linkDTO{Kind: string(l.Kind), Repo: l.Repo, Ref: l.Ref, URL: l.URL, Title: l.Title, State: l.State, UpdatedAt: l.UpdatedAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// GitHub's webhook payloads (only the fields Kyber reads).
type githubRepo struct {
	FullName string `json:"full_name"`
}

type githubPush struct {
	Repository githubRepo `json:"repository"`
	Commits    []struct {
		ID      string `json:"id"`
		URL     string `json:"url"`
		Message string `json:"message"`
	} `json:"commits"`
}

type githubPR struct {
	Repository  githubRepo `json:"repository"`
	PullRequest struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
		State   string `json:"state"`
		Merged  bool   `json:"merged"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

// githubHook records commits (push) and pull requests (pull_request) that mention issue keys.
// The body must carry a valid X-Hub-Signature-256 for the configured secret; other events
// (ping included) are acknowledged and ignored.
func (h *Handler) githubHook(w http.ResponseWriter, r *http.Request) {
	if h.github == "" {
		httpx.Error(w, r, h.log, errDisabled, codeFor)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGitHubBody))
	if err != nil {
		httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
		return
	}
	if !domain.Verify(h.github, body, r.Header.Get("X-Hub-Signature-256")) {
		httpx.Error(w, r, h.log, errSignature, codeFor)
		return
	}
	linked := 0
	switch r.Header.Get("X-GitHub-Event") {
	case "push":
		var p githubPush
		if json.Unmarshal(body, &p) != nil {
			httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
			return
		}
		push := application.Push{Repo: p.Repository.FullName}
		for _, c := range p.Commits {
			push.Commits = append(push.Commits, application.Commit{SHA: c.ID, URL: c.URL, Message: c.Message})
		}
		linked, err = h.svc.RecordPush(r.Context(), push)
	case "pull_request":
		var p githubPR
		if json.Unmarshal(body, &p) != nil {
			httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
			return
		}
		pr := p.PullRequest
		linked, err = h.svc.RecordPullRequest(r.Context(), application.PullRequest{Repo: p.Repository.FullName, Number: pr.Number,
			URL: pr.HTMLURL, Title: pr.Title, Branch: pr.Head.Ref, State: pr.State, Merged: pr.Merged})
	}
	if errors.Is(err, domain.ErrInvalidLink) {
		err = nil // a malformed commit or PR is skipped, not retried by GitHub forever
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"linked": linked})
}
