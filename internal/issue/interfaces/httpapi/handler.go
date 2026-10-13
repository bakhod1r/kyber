// Package httpapi exposes the Issue Tracking context over REST.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{key}/issues", h.create)
	mux.HandleFunc("GET /api/v1/projects/{key}/issues", h.list)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}", h.get)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/transitions", h.transition)
	mux.HandleFunc("PATCH /api/v1/issues/{issueKey}", h.edit)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/rank", h.rank)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/comments", h.comments)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/comments", h.addComment)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/history", h.history)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/watchers", h.watchers)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/worklogs", h.worklogs)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/worklogs", h.logWork)
	mux.HandleFunc("DELETE /api/v1/worklogs/{id}", h.deleteWorklog)
	mux.HandleFunc("GET /api/v1/projects/{key}/fields", h.customFields)
	mux.HandleFunc("POST /api/v1/projects/{key}/fields", h.createCustomField)
	mux.HandleFunc("PUT /api/v1/fields/{id}", h.editCustomField)
	mux.HandleFunc("DELETE /api/v1/fields/{id}", h.deleteCustomField)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/watchers", h.watch)
	mux.HandleFunc("DELETE /api/v1/issues/{issueKey}/watchers", h.unwatch)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/children", h.children)
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/links", h.links)
	mux.HandleFunc("POST /api/v1/issues/{issueKey}/links", h.link)
	mux.HandleFunc("DELETE /api/v1/links/{id}", h.unlink)
	mux.HandleFunc("GET /api/v1/projects/{key}/workflow", h.workflow)
	mux.HandleFunc("PUT /api/v1/projects/{key}/workflow", h.setWorkflow)
}

type issueDTO struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Title       string            `json:"title"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Priority    string            `json:"priority"`
	AssigneeID  *string           `json:"assignee_id"`
	SprintID    *string           `json:"sprint_id"`
	ReporterID  *string           `json:"reporter_id"`
	Estimate    *float64          `json:"estimate"`
	Rank        string            `json:"rank"`
	ParentKey   *string           `json:"parent_key"`
	Security    string            `json:"security_level"`
	Resolution  string            `json:"resolution"`
	FixVersions []string          `json:"fix_versions"`
	Time        timeDTO           `json:"time_tracking"`
	Fields      map[string]string `json:"custom_fields"`
	StartDate   *string           `json:"start_date"`
	DueDate     *string           `json:"due_date"`
	Version     int               `json:"version"`
}

func toDTO(is *domain.Issue) issueDTO {
	d := issueDTO{ID: string(is.ID()), Key: is.Key().String(), Title: is.Title(), Type: string(is.Type()),
		Status: string(is.Status()), Description: is.Description(), Priority: string(is.Priority()),
		Rank: string(is.Rank()), Security: is.SecurityLevel(), Resolution: string(is.Resolution()), FixVersions: fixVersions(is), Time: timeOut(is.TimeTracking()), Fields: fieldsOut(is), Version: is.Version()}
	if start, due := is.Schedule(); start != "" || due != "" {
		d.StartDate, d.DueDate = dateOut(start), dateOut(due)
	}
	if is.Parent() != "" {
		k := is.ParentKey().String()
		d.ParentKey = &k
	}
	if a := is.Assignee(); a != "" {
		s := string(a)
		d.AssigneeID = &s
	}
	if p, ok := is.Estimate(); ok {
		f := p.Float()
		d.Estimate = &f
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
	case errors.Is(err, application.ErrInvalidKey):
		return httpx.CodeInvalidIssueKey, true
	case errors.Is(err, domain.ErrEmptyTitle), errors.Is(err, domain.ErrInvalidTitle), errors.Is(err, domain.ErrInvalidIssueType),
		errors.Is(err, domain.ErrInvalidPriority), errors.Is(err, domain.ErrDescriptionTooLong),
		errors.Is(err, domain.ErrInvalidCommentBody), errors.Is(err, application.ErrInvalidAssignee), errors.Is(err, errMissingVersion),
		errors.Is(err, application.ErrInvalidSprint), errors.Is(err, application.ErrInvalidAnchor), errors.Is(err, domain.ErrInvalidEstimate),
		errors.Is(err, domain.ErrInvalidWorkflow), errors.Is(err, domain.ErrUnknownStatus),
		errors.Is(err, application.ErrInvalidSecurityLevel), errors.Is(err, domain.ErrInvalidResolution), errors.Is(err, application.ErrInvalidVersion),
		errors.Is(err, domain.ErrInvalidDuration), errors.Is(err, domain.ErrInvalidWorklog),
		errors.Is(err, domain.ErrInvalidCustomField), errors.Is(err, domain.ErrInvalidFieldValue), errors.Is(err, domain.ErrInvalidSchedule):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrFieldNotFound):
		return httpx.CodeFieldNotFound, true
	case errors.Is(err, domain.ErrFieldConflict):
		return httpx.CodeFieldConflict, true
	case errors.Is(err, domain.ErrWorklogNotFound):
		return httpx.CodeWorklogNotFound, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, domain.ErrIssueNotFound):
		return httpx.CodeIssueNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, domain.ErrTransitionNotAllowed):
		return httpx.CodeTransitionNotAllowed, true
	case errors.Is(err, domain.ErrInvalidLink):
		return httpx.CodeInvalidLink, true
	case errors.Is(err, domain.ErrLinkExists):
		return httpx.CodeLinkExists, true
	case errors.Is(err, domain.ErrLinkNotFound):
		return httpx.CodeLinkNotFound, true
	case errors.Is(err, domain.ErrInvalidParent):
		return httpx.CodeInvalidParent, true
	case errors.Is(err, application.ErrStatusInUse):
		return httpx.CodeStatusInUse, true
	case errors.Is(err, domain.ErrConcurrentModification):
		return httpx.CodeConcurrentModified, true
	}
	return "", false
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title, Type string
		ParentKey   string `json:"parent_key"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), application.CreateIssue{Project: r.PathValue("key"), Title: in.Title, Type: in.Type, Parent: in.ParentKey})
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
	var in struct{ To, Resolution string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Transition(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"), in.To, in.Resolution)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("key"),
		application.ListQuery{Status: r.URL.Query().Get("status"), Sprint: r.URL.Query().Get("sprint"), FixVersion: r.URL.Query().Get("fix_version")})
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
		Version     *int              `json:"version"`
		Title       *string           `json:"title"`
		Description *string           `json:"description"`
		Priority    *string           `json:"priority"`
		AssigneeID  json.RawMessage   `json:"assignee_id"`
		SprintID    json.RawMessage   `json:"sprint_id"`
		Estimate    json.RawMessage   `json:"estimate"`
		ParentKey   json.RawMessage   `json:"parent_key"`
		Security    *string           `json:"security_level"`
		Resolution  *string           `json:"resolution"`
		FixVersions *[]string         `json:"fix_versions"`
		Original    *string           `json:"original_estimate"`
		Remaining   *string           `json:"remaining_estimate"`
		Fields      map[string]string `json:"custom_fields"`
		StartDate   *string           `json:"start_date"`
		DueDate     *string           `json:"due_date"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if in.Version == nil {
		httpx.Error(w, r, h.log, errMissingVersion, codeFor)
		return
	}
	cmd := application.EditIssue{Version: *in.Version, Title: in.Title, Description: in.Description, Priority: in.Priority, Security: in.Security, Resolution: in.Resolution, FixVersions: in.FixVersions,
		OriginalEstimate: in.Original, RemainingEstimate: in.Remaining, Fields: in.Fields, StartDate: in.StartDate, DueDate: in.DueDate}
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
	if in.Estimate != nil {
		cmd.EstimateSet = true
		if string(in.Estimate) != "null" {
			var f float64
			if err := json.Unmarshal(in.Estimate, &f); err != nil {
				httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
				return
			}
			cmd.Estimate = &f
		}
	}
	if in.ParentKey != nil {
		parent := ""
		if string(in.ParentKey) != "null" {
			if err := json.Unmarshal(in.ParentKey, &parent); err != nil {
				httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
				return
			}
		}
		cmd.Parent = &parent
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

func toCommentDTO(c application.CommentView) commentDTO {
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
	httpx.JSON(w, http.StatusCreated, toCommentDTO(application.CommentView{Comment: c, AuthorName: name}))
}

func (h *Handler) rank(w http.ResponseWriter, r *http.Request) {
	var in struct{ After, Before string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	is, err := h.svc.Rank(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"), application.RankMove{After: in.After, Before: in.Before})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(is))
}

func (h *Handler) children(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Children(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]issueDTO, 0, len(c.Items))
	for _, is := range c.Items {
		items = append(items, toDTO(is))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": len(c.Items), "done": c.Done,
		"points": float64(c.Points) / 10, "done_points": float64(c.DonePoints) / 10})
}

func fixVersions(is *domain.Issue) []string {
	out := []string{}
	for _, v := range is.FixVersions() {
		out = append(out, string(v))
	}
	return out
}

type watcherDTO struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

func (h *Handler) watchers(w http.ResponseWriter, r *http.Request) {
	list, watching, err := h.svc.Watchers(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]watcherDTO, 0, len(list))
	for _, x := range list {
		out = append(out, watcherDTO{UserID: x.ID, Name: x.Name})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"watching": watching, "items": out})
}

func (h *Handler) watch(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Watch(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unwatch(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unwatch(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// dateOut renders an unset date as null.
func dateOut(d domain.Date) *string {
	if d == "" {
		return nil
	}
	s := string(d)
	return &s
}
