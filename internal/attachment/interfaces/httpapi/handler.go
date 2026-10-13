// Package httpapi serves issue attachments: multipart upload, listing, download and removal.
package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/attachment/application"
	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// errNoFile: the multipart body had no "file" part.
var errNoFile = errors.New("missing file part")

// maxBody leaves room for multipart framing around the largest file.
const maxBody = domain.MaxSize + 1<<20

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/issues/{key}/attachments", h.list)
	mux.HandleFunc("POST /api/v1/issues/{key}/attachments", h.upload)
	mux.HandleFunc("GET /api/v1/attachments/{id}", h.download)
	mux.HandleFunc("DELETE /api/v1/attachments/{id}", h.delete)
}

func codeFor(err error) (string, bool) {
	var tooBig *http.MaxBytesError
	switch {
	case errors.Is(err, domain.ErrTooLarge), errors.As(err, &tooBig):
		return httpx.CodeAttachmentTooLarge, true
	case errors.Is(err, domain.ErrInvalidAttachment), errors.Is(err, errNoFile):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrAttachmentNotFound):
		return httpx.CodeAttachmentNotFound, true
	case errors.Is(err, application.ErrIssueNotFound):
		return httpx.CodeIssueNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	}
	return "", false
}

type attachmentDTO struct {
	ID          string    `json:"id"`
	IssueKey    string    `json:"issue_key"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	UploadedBy  string    `json:"uploaded_by"`
	UploadedAt  time.Time `json:"uploaded_at"`
	Inline      bool      `json:"inline"`
	URL         string    `json:"url"`
}

func out(a *domain.Attachment) attachmentDTO {
	return attachmentDTO{ID: string(a.ID), IssueKey: a.Issue, Name: a.Name, ContentType: a.ContentType, Size: a.Size,
		SHA256: a.SHA256, UploadedBy: a.UploadedBy, UploadedAt: a.UploadedAt, Inline: a.Inline(), URL: "/api/v1/attachments/" + string(a.ID)}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]attachmentDTO, 0, len(list))
	for _, a := range list {
		items = append(items, out(a))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// upload streams the multipart "file" part straight to the blob store.
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, r, h.log, fmt.Errorf("%w: %w", domain.ErrInvalidAttachment, err), codeFor)
		return
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			httpx.Error(w, r, h.log, errNoFile, codeFor)
			return
		}
		if err != nil {
			httpx.Error(w, r, h.log, fmt.Errorf("%w: %w", domain.ErrInvalidAttachment, err), codeFor)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		a, err := h.svc.Upload(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), part.FileName(), part.Header.Get("Content-Type"), part)
		if err != nil {
			httpx.Error(w, r, h.log, err, codeFor)
			return
		}
		httpx.JSON(w, http.StatusCreated, out(a))
		return
	}
}

// download serves the file. Only images and PDFs may render in place (?download forces a
// download); everything else is an attachment, sandboxed and never sniffed, so uploaded HTML or
// SVG cannot run in Kyber's origin.
func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	a, body, err := h.svc.Open(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	defer body.Close()
	disposition := "attachment"
	if a.Inline() && !r.URL.Query().Has("download") {
		disposition = "inline"
	}
	hd := w.Header()
	hd.Set("Content-Type", a.ContentType)
	hd.Set("Content-Length", strconv.FormatInt(a.Size, 10))
	hd.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Name}))
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Cache-Control", "private, max-age=0, must-revalidate")
	hd.Set("ETag", `"`+a.SHA256+`"`)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		h.log.WarnContext(r.Context(), "attachment download interrupted", "id", a.ID, "err", err)
	}
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
