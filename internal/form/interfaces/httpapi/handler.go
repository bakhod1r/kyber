// Package httpapi serves a project's forms and their submissions.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bakhod1r/kyber/internal/form/application"
	"github.com/bakhod1r/kyber/internal/form/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/forms", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/forms", h.create)
	mux.HandleFunc("GET /api/v1/forms/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/forms/{id}", h.redesign)
	mux.HandleFunc("DELETE /api/v1/forms/{id}", h.delete)
	mux.HandleFunc("POST /api/v1/forms/{id}/submissions", h.submit)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidForm), errors.Is(err, domain.ErrInvalidSubmission), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrFormNotFound):
		return httpx.CodeFormNotFound, true
	case errors.Is(err, domain.ErrFormConflict):
		return httpx.CodeFormConflict, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type questionDTO struct {
	ID       string `json:"id,omitempty"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Long     bool   `json:"long"`
}

type formDTO struct {
	ID         string        `json:"id"`
	ProjectKey string        `json:"project_key"`
	Name       string        `json:"name"`
	Intro      string        `json:"intro"`
	IssueType  string        `json:"issue_type"`
	Questions  []questionDTO `json:"questions"`
	Version    int           `json:"version"`
}

func out(f *domain.Form) formDTO {
	qs := make([]questionDTO, len(f.Questions))
	for i, q := range f.Questions {
		qs[i] = questionDTO(q)
	}
	return formDTO{ID: string(f.ID), ProjectKey: f.Project, Name: f.Name, Intro: f.Intro, IssueType: f.IssueType, Questions: qs, Version: f.Version}
}

type formIn struct {
	Name      string        `json:"name"`
	Intro     string        `json:"intro"`
	IssueType string        `json:"issue_type"`
	Questions []questionDTO `json:"questions"`
	Version   int           `json:"version"`
}

func (in formIn) design() application.Design {
	qs := make([]domain.QuestionInput, len(in.Questions))
	for i, q := range in.Questions {
		qs[i] = domain.QuestionInput{Label: q.Label, Required: q.Required, Long: q.Long}
	}
	return application.Design{Name: in.Name, Intro: in.Intro, IssueType: in.IssueType, Questions: qs}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Forms(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]formDTO, 0, len(list))
	for _, f := range list {
		items = append(items, out(f))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in formIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	f, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.design())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(f))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	f, err := h.svc.Form(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(f))
}

func (h *Handler) redesign(w http.ResponseWriter, r *http.Request) {
	var in formIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	f, err := h.svc.Redesign(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Version, in.design())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(f))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Summary string            `json:"summary"`
		Answers map[string]string `json:"answers"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	key, err := h.svc.Submit(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Summary, in.Answers)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"issue_key": key})
}
