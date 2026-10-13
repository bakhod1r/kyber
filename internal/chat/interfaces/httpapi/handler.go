// Package httpapi exposes team chat (channels, DMs, messages) over REST.
package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	app "github.com/bakhod1r/kyber/internal/chat/application"
	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/conversations", h.list)
	mux.HandleFunc("POST /api/v1/channels", h.createChannel)
	mux.HandleFunc("POST /api/v1/dms", h.openDM)
	mux.HandleFunc("GET /api/v1/conversations/{id}", h.get)
	mux.HandleFunc("POST /api/v1/conversations/{id}/join", h.join)
	mux.HandleFunc("POST /api/v1/conversations/{id}/leave", h.leave)
	mux.HandleFunc("POST /api/v1/conversations/{id}/members", h.invite)
	mux.HandleFunc("POST /api/v1/conversations/{id}/read", h.read)
	mux.HandleFunc("POST /api/v1/conversations/{id}/typing", h.typing)
	mux.HandleFunc("GET /api/v1/conversations/{id}/messages", h.history)
	mux.HandleFunc("POST /api/v1/conversations/{id}/messages", h.post)
	mux.HandleFunc("PATCH /api/v1/messages/{id}", h.edit)
	mux.HandleFunc("DELETE /api/v1/messages/{id}", h.delete)
	mux.HandleFunc("GET /api/v1/messages/{id}/replies", h.replies)
	mux.HandleFunc("POST /api/v1/messages/{id}/replies", h.reply)
	mux.HandleFunc("POST /api/v1/messages/{id}/reactions", h.react)
	mux.HandleFunc("POST /api/v1/messages/{id}/pin", h.pin)
	mux.HandleFunc("DELETE /api/v1/messages/{id}/pin", h.unpin)
	mux.HandleFunc("GET /api/v1/conversations/{id}/pins", h.pins)
	mux.HandleFunc("GET /api/v1/chat/search", h.search)
	mux.HandleFunc("POST /api/v1/conversations/{id}/files", h.upload)
	mux.HandleFunc("GET /api/v1/messages/{id}/file", h.download)
}

var errBadLimit = errors.New("limit must be a number from 1 to 200")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrConversationNotFound):
		return httpx.CodeConversationNotFound, true
	case errors.Is(err, domain.ErrMessageNotFound):
		return httpx.CodeMessageNotFound, true
	case errors.Is(err, domain.ErrNameTaken):
		return httpx.CodeChannelNameTaken, true
	case errors.Is(err, domain.ErrNotMember), errors.Is(err, domain.ErrNotAuthor), errors.Is(err, domain.ErrNotJoinable),
		errors.Is(err, domain.ErrDeleted):
		return httpx.CodeChatForbidden, true
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidBody), errors.Is(err, domain.ErrDMSize),
		errors.Is(err, app.ErrNotWorkspaceMember), errors.Is(err, errBadLimit), errors.Is(err, domain.ErrNestedThread),
		errors.Is(err, domain.ErrInvalidEmoji), errors.Is(err, app.ErrQueryTooShort), errors.Is(err, domain.ErrInvalidFile):
		return httpx.CodeValidation, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

type conversationDTO struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Topic     string    `json:"topic"`
	Private   bool      `json:"private"`
	MemberIDs []string  `json:"member_ids"`
	Member    bool      `json:"member"`
	Unread    int       `json:"unread"`
	CreatedAt time.Time `json:"created_at"`
}

func toConversation(v app.View, actor string) conversationDTO {
	d := conversationDTO{ID: string(v.ID), Kind: string(v.Kind), Name: v.Name, Topic: v.Topic, Private: v.Private,
		MemberIDs: make([]string, len(v.Members)), Member: v.IsMember(domain.UserID(actor)), Unread: v.Unread, CreatedAt: v.CreatedAt}
	for i, m := range v.Members {
		d.MemberIDs[i] = string(m)
	}
	return d
}

type messageDTO struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	AuthorID       string     `json:"author_id"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
	EditedAt       *time.Time `json:"edited_at"`
	Deleted        bool       `json:"deleted"`
	ThreadID       *string    `json:"thread_id"`
	ReplyCount     int        `json:"reply_count"`
	Kind           string     `json:"kind"`
	File           *fileDTO   `json:"file"`
	PinnedBy       *string    `json:"pinned_by"`
	PinnedAt       *time.Time `json:"pinned_at"`
	Reactions      []reaction `json:"reactions"`
	Issues         []issueDTO `json:"issues"`
}

type issueDTO struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type fileDTO struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

type reaction struct {
	Emoji   string   `json:"emoji"`
	UserIDs []string `json:"user_ids"`
}

func toMessage(m *domain.Message) messageDTO {
	d := messageDTO{ID: string(m.ID), ConversationID: string(m.Conversation), AuthorID: string(m.Author), Body: m.Body,
		CreatedAt: m.CreatedAt, Deleted: m.Deleted, ReplyCount: m.ReplyCount, Kind: string(m.Kind), Reactions: []reaction{}}
	if !m.EditedAt.IsZero() {
		d.EditedAt = &m.EditedAt
	}
	if m.Thread != "" {
		t := string(m.Thread)
		d.ThreadID = &t
	}
	if f := m.File; f != nil {
		d.File = &fileDTO{Name: f.Name, Type: f.Type, Size: f.Size, URL: "/api/v1/messages/" + string(m.ID) + "/file"}
	}
	if m.PinnedBy != "" {
		by, at := string(m.PinnedBy), m.PinnedAt
		d.PinnedBy, d.PinnedAt = &by, &at
	}
	for _, r := range m.Reactions {
		users := make([]string, len(r.Users))
		for i, u := range r.Users {
			users[i] = string(u)
		}
		d.Reactions = append(d.Reactions, reaction{Emoji: r.Emoji, UserIDs: users})
	}
	return d
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Error(w, r, h.log, err, codeFor)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor := auth.Actor(r.Context())
	views, err := h.svc.Conversations(r.Context(), actor)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]conversationDTO, len(views))
	for i, v := range views {
		out[i] = toConversation(v, actor)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name    string `json:"name"`
		Topic   string `json:"topic"`
		Private bool   `json:"private"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	actor := auth.Actor(r.Context())
	c, err := h.svc.CreateChannel(r.Context(), actor, app.NewChannel{Name: in.Name, Topic: in.Topic, Private: in.Private})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toConversation(app.View{Conversation: c}, actor))
}

func (h *Handler) openDM(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	actor := auth.Actor(r.Context())
	c, err := h.svc.OpenDM(r.Context(), actor, in.UserIDs)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toConversation(app.View{Conversation: c}, actor))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor := auth.Actor(r.Context())
	v, err := h.svc.Conversation(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toConversation(v, actor))
}

// noContent runs an action on the conversation in the path and answers 204.
func (h *Handler) noContent(w http.ResponseWriter, r *http.Request, do func() error) {
	if err := do(); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) join(w http.ResponseWriter, r *http.Request) {
	h.noContent(w, r, func() error { return h.svc.Join(r.Context(), auth.Actor(r.Context()), r.PathValue("id")) })
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	h.noContent(w, r, func() error { return h.svc.Leave(r.Context(), auth.Actor(r.Context()), r.PathValue("id")) })
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	h.noContent(w, r, func() error { return h.svc.MarkRead(r.Context(), auth.Actor(r.Context()), r.PathValue("id")) })
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	h.noContent(w, r, func() error { return h.svc.Invite(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.UserID) })
}

// history: ?before=<message id>&limit= (newest first).
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > app.MaxPage {
			h.fail(w, r, errBadLimit)
			return
		}
		limit = n
	}
	ms, err := h.svc.History(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), q.Get("before"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.messages(w, r, ms)
}

// render turns messages into DTOs with the issue previews the caller may see.
func (h *Handler) render(r *http.Request, ms ...*domain.Message) ([]messageDTO, error) {
	previews, err := h.svc.Previews(r.Context(), auth.Actor(r.Context()), ms)
	if err != nil {
		return nil, err
	}
	out := make([]messageDTO, len(ms))
	for i, m := range ms {
		out[i] = toMessage(m)
		out[i].Issues = []issueDTO{}
		for _, p := range previews[m.ID] {
			out[i].Issues = append(out[i].Issues, issueDTO{Key: p.Key, Title: p.Title, Status: p.Status})
		}
	}
	return out, nil
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request, ms []*domain.Message) {
	out, err := h.render(r, ms...)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) message(w http.ResponseWriter, r *http.Request, status int, m *domain.Message) {
	out, err := h.render(r, m)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, status, out[0])
}

func decodeBody(r *http.Request) (string, error) {
	var in struct {
		Body string `json:"body"`
	}
	err := httpx.Decode(r, &in)
	return in.Body, err
}

func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Post(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusCreated, m)
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Edit(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusOK, m)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	h.noContent(w, r, func() error { return h.svc.Delete(r.Context(), auth.Actor(r.Context()), r.PathValue("id")) })
}

func (h *Handler) typing(w http.ResponseWriter, r *http.Request) {
	h.noContent(w, r, func() error { return h.svc.Typing(r.Context(), auth.Actor(r.Context()), r.PathValue("id")) })
}

func (h *Handler) replies(w http.ResponseWriter, r *http.Request) {
	ms, err := h.svc.Replies(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.messages(w, r, ms)
}

func (h *Handler) reply(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Reply(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusCreated, m)
}

func (h *Handler) react(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Emoji string `json:"emoji"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.React(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Emoji)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusOK, m)
}

func (h *Handler) pin(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.Pin(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusOK, m)
}

func (h *Handler) unpin(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.Unpin(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusOK, m)
}

func (h *Handler) pins(w http.ResponseWriter, r *http.Request) {
	ms, err := h.svc.Pins(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.messages(w, r, ms)
}

// search: ?q=words (every word must appear; case-insensitive).
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	ms, err := h.svc.Search(r.Context(), auth.Actor(r.Context()), r.URL.Query().Get("q"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.messages(w, r, ms)
}

// upload: the request body is the file (≤ 25 MB), ?name= its name.
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, domain.MaxFile+1)
	m, err := h.svc.Upload(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.URL.Query().Get("name"), body)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		err = domain.ErrInvalidFile
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.message(w, r, http.StatusCreated, m)
}

// inline types are shown in the browser; everything else (HTML, SVG, scripts…) is only downloaded.
func inline(typ string) bool {
	switch {
	case typ == "image/svg+xml":
		return false
	case strings.HasPrefix(typ, "image/"), strings.HasPrefix(typ, "video/"), strings.HasPrefix(typ, "audio/"), typ == "application/pdf":
		return true
	}
	return false
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	f, body, err := h.svc.Download(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer body.Close()
	disposition := "attachment"
	if inline(f.Type) {
		disposition = "inline"
	}
	hd := w.Header()
	hd.Set("Content-Type", f.Type)
	hd.Set("Content-Length", strconv.FormatInt(f.Size, 10))
	hd.Set("Content-Disposition", disposition+"; filename*=UTF-8''"+url.PathEscape(f.Name))
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; media-src 'self'")
	hd.Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}
