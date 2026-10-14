// Package httpapi serves a project's pages and their history.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/application"
	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/docs", h.list)
	mux.HandleFunc("POST /api/v1/projects/{key}/docs", h.create)
	mux.HandleFunc("GET /api/v1/docs/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/docs/{id}", h.revise)
	mux.HandleFunc("DELETE /api/v1/docs/{id}", h.delete)
	mux.HandleFunc("GET /api/v1/docs/{id}/history", h.history)
	mux.HandleFunc("GET /api/v1/docs/{id}/comments", h.comments)
	mux.HandleFunc("POST /api/v1/docs/{id}/comments", h.comment)
	mux.HandleFunc("DELETE /api/v1/doc-comments/{id}", h.removeComment)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidPage), errors.Is(err, domain.ErrInvalidComment), errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrPageNotFound):
		return httpx.CodePageNotFound, true
	case errors.Is(err, domain.ErrPageConflict):
		return httpx.CodePageConflict, true
	case errors.Is(err, application.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, domain.ErrCommentMissing):
		return httpx.CodeCommentNotFound, true
	case errors.Is(err, application.ErrNotAuthor):
		return httpx.CodeNotCommentAuthor, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type pageDTO struct {
	ID         string    `json:"id"`
	ProjectKey string    `json:"project_key"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	AuthorID   string    `json:"author_id"`
	UpdatedBy  string    `json:"updated_by"`
	UpdatedAt  time.Time `json:"updated_at"`
	Version    int       `json:"version"`
}

func out(p *domain.Page) pageDTO {
	return pageDTO{ID: string(p.ID), ProjectKey: p.Project, Title: p.Title, Body: p.Body, AuthorID: p.Author,
		UpdatedBy: p.UpdatedBy, UpdatedAt: p.UpdatedAt, Version: p.Version}
}

type pageIn struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Version int    `json:"version"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Pages(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]pageDTO, 0, len(list))
	for _, p := range list {
		items = append(items, out(p))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in pageIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	p, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.Title, in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, out(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Page(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(p))
}

func (h *Handler) revise(w http.ResponseWriter, r *http.Request) {
	var in pageIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	p, err := h.svc.Revise(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Version, in.Title, in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, out(p))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type revisionDTO struct {
	Number int       `json:"number"`
	Title  string    `json:"title"`
	Body   string    `json:"body"`
	By     string    `json:"by"`
	At     time.Time `json:"at"`
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.History(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]revisionDTO, 0, len(list))
	for _, v := range list {
		items = append(items, revisionDTO{Number: v.Number, Title: v.Title, Body: v.Body, By: v.By, At: v.At})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type commentDTO struct {
	ID        string    `json:"id"`
	PageID    string    `json:"page_id"`
	Paragraph int       `json:"paragraph"`
	Quote     string    `json:"quote"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func commentOut(c *domain.Comment) commentDTO {
	return commentDTO{ID: string(c.ID), PageID: string(c.Page), Paragraph: c.Paragraph, Quote: c.Quote, AuthorID: c.Author, Body: c.Body, CreatedAt: c.CreatedAt}
}

func (h *Handler) comments(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Comments(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]commentDTO, 0, len(list))
	for _, c := range list {
		items = append(items, commentOut(c))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) comment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paragraph int    `json:"paragraph"`
		Quote     string `json:"quote"`
		Body      string `json:"body"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	c, err := h.svc.Comment(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Paragraph, in.Quote, in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, commentOut(c))
}

func (h *Handler) removeComment(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveComment(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
