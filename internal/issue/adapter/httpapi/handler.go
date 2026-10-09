// Package httpapi exposes the Issue Tracking context over REST.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{key}/issues", h.create)
	mux.HandleFunc("GET /api/v1/projects/{key}/issues", h.list)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}", h.get)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/transitions", h.transition)
	mux.HandleFunc("PATCH /api/v1/issues/{issueKey}", h.edit)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/rank", h.rank)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/comments", h.comments)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/comments", h.addComment)
}

type issueDTO struct {
	ID          string  `json:"id"`
	Key         string  `json:"key"`
	Title       string  `json:"title"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	SprintID    *string `json:"sprint_id"`
	ReporterID  *string `json:"reporter_id"`
	Rank        string  `json:"rank"`
	Version     int     `json:"version"`
}

func toDTO(is *domain.Issue) issueDTO {
	d := issueDTO{ID: string(is.ID()), Key: is.Key().String(), Title: is.Title(), Type: string(is.Type()),
		Status: string(is.Status()), Description: is.Description(), Priority: string(is.Priority()),
		Rank: string(is.Rank()), Version: is.Version()}
	if a := is.Assignee(); a != "" {
		s := string(a)
		d.AssigneeID = &s
	}
	if r := is.Reporter(); r != "" {
		s := string(r)
		d.ReporterID = &s
	}
	if sp := is.Sprint(); sp != "" {
		s := string(sp)
		d.SprintID = &s
	}
	return d
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrInvalidKey):
		return httpx.CodeInvalidIssueKey, true
	case errors.Is(err, domain.ErrEmptyTitle), errors.Is(err, domain.ErrInvalidIssueType),
		errors.Is(err, domain.ErrInvalidPriority), errors.Is(err, domain.ErrDescriptionTooLong),
		errors.Is(err, domain.ErrInvalidCommentBody), errors.Is(err, app.ErrInvalidAssignee), errors.Is(err, errMissingVersion),
		errors.Is(err, app.ErrInvalidSprint), errors.Is(err, app.ErrInvalidAnchor):
		return httpx.CodeValidation, true
	case errors.Is(err, app.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, domain.ErrIssueNotFound):
		return httpx.CodeIssueNotFound, true
	case errors.Is(err, app.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, domain.ErrTransitionNotAllowed):
		return httpx.CodeTransitionNotAllowed, true
	case errors.Is(err, domain.ErrConcurrentModification):
		return httpx.CodeConcurrentModified, true
	}
	return "", false
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Title, Type string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), app.CreateIssue{Project: r.PathValue("key"), Title: in.Title, Type: in.Type})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(is))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	is, err := h.svc.Get(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request) {
	var in struct{ To string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Transition(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"), in.To)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("key"),
		app.ListQuery{Status: r.URL.Query().Get("status"), Sprint: r.URL.Query().Get("sprint")})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]issueDTO, 0, len(list))
	for _, is := range list {
		items = append(items, toDTO(is))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

var errMissingVersion = errors.New("version is required")

// edit accepts a partial update; "assignee_id": null unassigns, an absent key leaves it unchanged.
func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version     *int            `json:"version"`
		Title       *string         `json:"title"`
		Description *string         `json:"description"`
		Priority    *string         `json:"priority"`
		AssigneeID  json.RawMessage `json:"assignee_id"`
		SprintID    json.RawMessage `json:"sprint_id"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if in.Version == nil {
		httpx.Error(w, r, h.log, errMissingVersion, codeFor)
		return
	}
	cmd := app.EditIssue{Version: *in.Version, Title: in.Title, Description: in.Description, Priority: in.Priority}
	if in.AssigneeID != nil {
		cmd.AssigneeSet = true
		if string(in.AssigneeID) != "null" {
			if err := json.Unmarshal(in.AssigneeID, &cmd.Assignee); err != nil {
				httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
				return
			}
		}
	}
	if in.SprintID != nil {
		cmd.SprintSet = true
		if string(in.SprintID) != "null" {
			if err := json.Unmarshal(in.SprintID, &cmd.Sprint); err != nil {
				httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
				return
			}
		}
	}
	is, err := h.svc.Edit(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"), cmd)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

type commentDTO struct {
	ID         string    `json:"id"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

func toCommentDTO(c app.CommentView) commentDTO {
	return commentDTO{ID: string(c.ID()), AuthorID: string(c.Author()), AuthorName: c.AuthorName,
		Body: c.Body(), CreatedAt: c.CreatedAt().UTC()}
}

func (h *Handler) comments(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Comments(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]commentDTO, 0, len(list))
	for _, c := range list {
		items = append(items, toCommentDTO(c))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) addComment(w http.ResponseWriter, r *http.Request) {
	var in struct{ Body string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	c, err := h.svc.AddComment(r.Context(), actor, r.PathValue("issueKey"), in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	name, err := h.svc.DisplayName(r.Context(), actor)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toCommentDTO(app.CommentView{Comment: c, AuthorName: name}))
}

func (h *Handler) rank(w http.ResponseWriter, r *http.Request) {
	var in struct{ After, Before string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Rank(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"), app.RankMove{After: in.After, Before: in.Before})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}
