// Package httpapi exposes whiteboards over REST.
package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/whiteboard/application"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/conversations/{id}/whiteboards", h.list)
	mux.HandleFunc("POST /api/v1/conversations/{id}/whiteboards", h.create)
	mux.HandleFunc("GET /api/v1/whiteboards/{id}", h.get)
	mux.HandleFunc("POST /api/v1/whiteboards/{id}/ops", h.ops)
	mux.HandleFunc("POST /api/v1/whiteboards/{id}/cursor", h.cursor)
	mux.HandleFunc("GET /api/v1/whiteboards/{id}/comments", h.comments)
	mux.HandleFunc("POST /api/v1/whiteboards/{id}/comments", h.comment)
	mux.HandleFunc("DELETE /api/v1/whiteboard-comments/{id}", h.removeComment)
	mux.HandleFunc("POST /api/v1/whiteboards/{id}/images", h.uploadImage)
	mux.HandleFunc("GET /api/v1/whiteboards/{id}/images/{image}", h.image)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrNoConversation):
		return httpx.CodeConversationNotFound, true
	case errors.Is(err, domain.ErrWhiteboardMissing):
		return httpx.CodeWhiteboardNotFound, true
	case errors.Is(err, domain.ErrCommentMissing):
		return httpx.CodeCommentNotFound, true
	case errors.Is(err, application.ErrNotAuthor):
		return httpx.CodeNotCommentAuthor, true
	case errors.Is(err, domain.ErrImageMissing), errors.Is(err, application.ErrNoImages):
		return httpx.CodeImageNotFound, true
	case errors.Is(err, domain.ErrInvalidTitle), errors.Is(err, application.ErrTooManyOps), errors.Is(err, domain.ErrInvalidComment), errors.Is(err, domain.ErrNotImage):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrStale):
		return httpx.CodeWhiteboardBusy, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

type PointDTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ElementDTO is an element on the wire (REST and the whiteboard.ops stream event).
type ElementDTO struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	X         float64    `json:"x"`
	Y         float64    `json:"y"`
	W         float64    `json:"w"`
	H         float64    `json:"h"`
	Points    []PointDTO `json:"points"`
	Text      string     `json:"text"`
	Color     string     `json:"color"`
	Stroke    int        `json:"stroke,omitempty"` // S91: 1 thin, 2 medium, 3 bold; 0 = default
	Fill      bool       `json:"fill,omitempty"`
	Dashed    bool       `json:"dashed,omitempty"`
	Z         float64    `json:"z,omitempty"` // S93: layer order, lowest first
	Version   int        `json:"version"`
	UpdatedBy string     `json:"updated_by,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Deleted   bool       `json:"deleted,omitempty"`
}

func ToDTO(e domain.Element) ElementDTO {
	d := ElementDTO{ID: string(e.ID), Kind: string(e.Kind), X: e.X, Y: e.Y, W: e.W, H: e.H, Points: []PointDTO{}, Text: e.Text,
		Color: e.Color, Stroke: int(e.Stroke), Fill: e.Fill, Dashed: e.Dashed, Z: e.Z, Version: e.Version, UpdatedBy: string(e.UpdatedBy), Deleted: e.Deleted}
	if !e.UpdatedAt.IsZero() {
		d.UpdatedAt = &e.UpdatedAt
	}
	for _, p := range e.Points {
		d.Points = append(d.Points, PointDTO(p))
	}
	return d
}

func fromDTO(d ElementDTO) domain.Element {
	e := domain.Element{ID: domain.ElementID(d.ID), Kind: domain.ElementKind(d.Kind), X: d.X, Y: d.Y, W: d.W, H: d.H, Text: d.Text,
		Color: d.Color, Stroke: domain.Stroke(d.Stroke), Fill: d.Fill, Dashed: d.Dashed, Z: d.Z, Version: d.Version}
	for _, p := range d.Points {
		e.Points = append(e.Points, domain.Point(p))
	}
	return e
}

type boardDTO struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversation_id"`
	Title          string       `json:"title"`
	CreatedBy      string       `json:"created_by"`
	CreatedAt      time.Time    `json:"created_at"`
	Elements       []ElementDTO `json:"elements,omitempty"`
}

func toBoard(b *domain.Whiteboard, withElements bool) boardDTO {
	d := boardDTO{ID: string(b.ID), ConversationID: string(b.Conversation), Title: b.Title, CreatedBy: string(b.CreatedBy), CreatedAt: b.CreatedAt}
	if withElements {
		d.Elements = []ElementDTO{}
		for _, e := range b.Elements {
			d.Elements = append(d.Elements, ToDTO(e))
		}
		slices.SortFunc(d.Elements, func(a, b ElementDTO) int { return a.UpdatedAt.Compare(*b.UpdatedAt) }) // drawing order
	}
	return d
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Error(w, r, h.log, err, codeFor)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	bs, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]boardDTO, len(bs))
	for i, b := range bs {
		out[i] = toBoard(b, false)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	b, err := h.svc.Create(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Title)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toBoard(b, true))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Get(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toBoard(b, true))
}

func reason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, domain.ErrStale):
		return "stale"
	case errors.Is(err, domain.ErrElementNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrElementExists):
		return "exists"
	case errors.Is(err, domain.ErrBoardFull):
		return "full"
	}
	return "invalid"
}

func (h *Handler) ops(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Ops []struct {
			Op      string     `json:"op"`
			Element ElementDTO `json:"element"`
		} `json:"ops"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	ops := make([]domain.Op, len(in.Ops))
	for i, o := range in.Ops {
		ops[i] = domain.Op{Kind: domain.OpKind(o.Op), Element: fromDTO(o.Element)}
	}
	res, err := h.svc.Apply(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), ops)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type result struct {
		Applied bool       `json:"applied"`
		Error   string     `json:"error"`
		Element ElementDTO `json:"element"`
	}
	out := make([]result, len(res))
	for i, x := range res {
		out[i] = result{Applied: x.Applied, Error: reason(x.Error), Element: ToDTO(x.Element)}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"results": out})
}

func (h *Handler) cursor(w http.ResponseWriter, r *http.Request) {
	var in struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Cursor(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.X, in.Y); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type commentDTO struct {
	ID        string    `json:"id"`
	ElementID string    `json:"element_id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func toComment(c *domain.Comment) commentDTO {
	return commentDTO{ID: string(c.ID), ElementID: string(c.Element), AuthorID: string(c.Author), Body: c.Body, CreatedAt: c.CreatedAt}
}

func (h *Handler) comments(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Comments(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]commentDTO, len(list))
	for i, c := range list {
		out[i] = toComment(c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// comment: {"element_id", "body"} (S86).
func (h *Handler) comment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ElementID string `json:"element_id"`
		Body      string `json:"body"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	c, err := h.svc.Comment(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.ElementID, in.Body)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toComment(c))
}

func (h *Handler) removeComment(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveComment(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// uploadImage: the request body is the picture (S96); the answer names it for an image element.
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	board := r.PathValue("id")
	id, err := h.svc.UploadImage(r.Context(), auth.Actor(r.Context()), board, http.MaxBytesReader(w, r.Body, domain.MaxImage+1))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		err = domain.ErrNotImage
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": id, "url": "/api/v1/whiteboards/" + board + "/images/" + id})
}

func (h *Handler) image(w http.ResponseWriter, r *http.Request) {
	body, typ, err := h.svc.Image(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.PathValue("image"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	defer body.Close()
	hd := w.Header()
	hd.Set("Content-Type", typ)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	hd.Set("Cache-Control", "private, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}
