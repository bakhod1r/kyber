// Package httpapi exposes team chat (channels, DMs, messages) over REST.
package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
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
	mux.HandleFunc("PUT /api/v1/messages/{id}/save", h.save)
	mux.HandleFunc("DELETE /api/v1/messages/{id}/save", h.unsave)
	mux.HandleFunc("GET /api/v1/chat/saved", h.saved)
	mux.HandleFunc("PUT /api/v1/me/status", h.setStatus)
	mux.HandleFunc("GET /api/v1/groups", h.groups)
	mux.HandleFunc("PUT /api/v1/conversations/{id}/prefs", h.setPref)
	mux.HandleFunc("GET /api/v1/chat/prefs", h.prefs)
	mux.HandleFunc("POST /api/v1/groups", h.createGroup)
	mux.HandleFunc("PATCH /api/v1/groups/{id}", h.updateGroup)
	mux.HandleFunc("DELETE /api/v1/groups/{id}", h.deleteGroup)
	mux.HandleFunc("PATCH /api/v1/conversations/{id}", h.rename)
	mux.HandleFunc("POST /api/v1/conversations/{id}/archive", h.archive)
	mux.HandleFunc("POST /api/v1/conversations/{id}/unarchive", h.unarchive)
	mux.HandleFunc("PUT /api/v1/conversations/{id}/admins/{user}", h.makeAdmin)
	mux.HandleFunc("DELETE /api/v1/conversations/{id}/admins/{user}", h.removeAdmin)
	mux.HandleFunc("DELETE /api/v1/conversations/{id}/members/{user}", h.removeMember)
	mux.HandleFunc("PUT /api/v1/conversations/{id}/project", h.linkProject)
	mux.HandleFunc("GET /api/v1/conversations/{id}/bookmarks", h.bookmarks)
	mux.HandleFunc("POST /api/v1/conversations/{id}/bookmarks", h.addBookmark)
	mux.HandleFunc("DELETE /api/v1/bookmarks/{id}", h.removeBookmark)
	mux.HandleFunc("GET /api/v1/conversations/{id}/workflows", h.workflows)
	mux.HandleFunc("POST /api/v1/conversations/{id}/workflows", h.addWorkflow)
	mux.HandleFunc("DELETE /api/v1/workflows/{id}", h.removeWorkflow)
	mux.HandleFunc("GET /api/v1/conversations/{id}/receipts", h.receipts)
	mux.HandleFunc("POST /api/v1/conversations/{id}/scheduled", h.schedule)
	mux.HandleFunc("GET /api/v1/chat/scheduled", h.scheduledList)
	mux.HandleFunc("PATCH /api/v1/scheduled/{id}", h.reschedule)
	mux.HandleFunc("DELETE /api/v1/scheduled/{id}", h.cancelScheduled)
	mux.HandleFunc("POST /api/v1/conversations/{id}/reminders", h.remind)
	mux.HandleFunc("GET /api/v1/chat/statuses", h.statuses)
	mux.HandleFunc("POST /api/v1/conversations/{id}/files", h.upload)
	mux.HandleFunc("GET /api/v1/messages/{id}/file", h.download)
}

var errBadLimit = errors.New("limit must be a number from 1 to 200")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrConversationNotFound):
		return httpx.CodeConversationNotFound, true
	case errors.Is(err, domain.ErrHandleTaken):
		return httpx.CodeGroupHandleTaken, true
	case errors.Is(err, domain.ErrWorkflowNotFound):
		return httpx.CodeWorkflowNotFound, true
	case errors.Is(err, domain.ErrBookmarkNotFound):
		return httpx.CodeBookmarkNotFound, true
	case errors.Is(err, domain.ErrGroupNotFound):
		return httpx.CodeGroupNotFound, true
	case errors.Is(err, domain.ErrScheduledNotFound):
		return httpx.CodeScheduledNotFound, true
	case errors.Is(err, domain.ErrMessageNotFound):
		return httpx.CodeMessageNotFound, true
	case errors.Is(err, domain.ErrNameTaken):
		return httpx.CodeChannelNameTaken, true
	case errors.Is(err, domain.ErrNotMember), errors.Is(err, domain.ErrNotAuthor), errors.Is(err, domain.ErrNotJoinable),
		errors.Is(err, domain.ErrDeleted), errors.Is(err, domain.ErrNotManager), errors.Is(err, domain.ErrNotOwner), errors.Is(err, domain.ErrArchived),
		errors.Is(err, app.ErrCannotBrowse):
		return httpx.CodeChatForbidden, true
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrInvalidBody), errors.Is(err, domain.ErrDMSize),
		errors.Is(err, app.ErrNotWorkspaceMember), errors.Is(err, errBadLimit), errors.Is(err, domain.ErrNestedThread),
		errors.Is(err, domain.ErrInvalidEmoji), errors.Is(err, app.ErrQueryTooShort), errors.Is(err, domain.ErrInvalidFile), errors.Is(err, domain.ErrInvalidReminder), errors.Is(err, domain.ErrInvalidStatus), errors.Is(err, domain.ErrInvalidSchedule), errors.Is(err, domain.ErrInvalidHandle), errors.Is(err, domain.ErrInvalidPref), errors.Is(err, domain.ErrInvalidProject), errors.Is(err, domain.ErrInvalidBookmark), errors.Is(err, domain.ErrTooManyBookmarks), errors.Is(err, domain.ErrInvalidWorkflow):
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
	OwnerID   string    `json:"owner_id"`
	Project   string    `json:"project_key"`
	AdminIDs  []string  `json:"admin_ids"`
	Archived  bool      `json:"archived"`
}

func toConversation(v app.View, actor string) conversationDTO {
	d := conversationDTO{ID: string(v.ID), Kind: string(v.Kind), Name: v.Name, Topic: v.Topic, Private: v.Private,
		MemberIDs: make([]string, len(v.Members)), Member: v.IsMember(domain.UserID(actor)), Unread: v.Unread, CreatedAt: v.CreatedAt,
		OwnerID: string(v.CreatedBy), AdminIDs: []string{}, Archived: v.Archived(), Project: v.Project}
	for i, m := range v.Members {
		d.MemberIDs[i] = string(m)
	}
	for _, a := range v.Admins {
		d.AdminIDs = append(d.AdminIDs, string(a))
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
	upload, limit := h.svc.Upload, int64(domain.MaxFile)
	if r.URL.Query().Get("recording") == "1" {
		upload, limit = h.svc.UploadRecording, domain.MaxRecording
	}
	body := http.MaxBytesReader(w, r.Body, limit+1)
	m, err := upload(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.URL.Query().Get("name"), body)
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

type savedDTO struct {
	Message  messageDTO `json:"message"`
	SavedAt  time.Time  `json:"saved_at"`
	RemindAt *time.Time `json:"remind_at"`
	Reminded bool       `json:"reminded"`
}

func toSaved(sv *domain.Saved, m messageDTO) savedDTO {
	d := savedDTO{Message: m, SavedAt: sv.SavedAt, Reminded: sv.Reminded}
	if !sv.RemindAt.IsZero() {
		d.RemindAt = &sv.RemindAt
	}
	return d
}

// save: {"remind_at": optional RFC 3339 time in the future}.
func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RemindAt *time.Time `json:"remind_at"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	var at time.Time
	if in.RemindAt != nil {
		at = *in.RemindAt
	}
	sv, err := h.svc.SaveMessage(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), at)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"saved_at": sv.SavedAt, "remind_at": in.RemindAt})
}

func (h *Handler) unsave(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unsave(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) saved(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.SavedFor(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ms := make([]*domain.Message, len(items))
	for i, it := range items {
		ms[i] = it.Message
	}
	rendered, err := h.render(r, ms...)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]savedDTO, len(items))
	for i, it := range items {
		out[i] = toSaved(it.Saved, rendered[i])
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type statusDTO struct {
	UserID    string     `json:"user_id"`
	Emoji     string     `json:"emoji"`
	Text      string     `json:"text"`
	ExpiresAt *time.Time `json:"expires_at"`
	DND       bool       `json:"dnd"`
	DNDUntil  *time.Time `json:"dnd_until"`
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func toStatus(user string, v domain.StatusView) statusDTO {
	return statusDTO{UserID: user, Emoji: v.Emoji, Text: v.Text, ExpiresAt: timeOrNil(v.ExpiresAt), DND: v.DND, DNDUntil: timeOrNil(v.DNDUntil)}
}

// setStatus: {"emoji", "text", "expires_at"?, "dnd_until"?}; empty emoji and text clear the status.
func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Emoji     string     `json:"emoji"`
		Text      string     `json:"text"`
		ExpiresAt *time.Time `json:"expires_at"`
		DNDUntil  *time.Time `json:"dnd_until"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	var exp, dnd time.Time
	if in.ExpiresAt != nil {
		exp = *in.ExpiresAt
	}
	if in.DNDUntil != nil {
		dnd = *in.DNDUntil
	}
	actor := auth.Actor(r.Context())
	st, err := h.svc.SetStatus(r.Context(), actor, in.Emoji, in.Text, exp, dnd)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toStatus(actor, st.At(time.Now())))
}

// statuses: ?user_id=a&user_id=b (at most 200); people without a status are left out.
func (h *Handler) statuses(w http.ResponseWriter, r *http.Request) {
	users := r.URL.Query()["user_id"]
	if len(users) > 200 {
		h.fail(w, r, errBadLimit)
		return
	}
	got, err := h.svc.Statuses(r.Context(), auth.Actor(r.Context()), users)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := []statusDTO{}
	for _, u := range users {
		if v, ok := got[u]; ok {
			out = append(out, toStatus(u, v))
			delete(got, u)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type scheduledDTO struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Body           string    `json:"body"`
	SendAt         time.Time `json:"send_at"`
	CreatedAt      time.Time `json:"created_at"`
}

func toScheduled(s *domain.Scheduled) scheduledDTO {
	return scheduledDTO{ID: string(s.ID), ConversationID: string(s.Conversation), Body: s.Body, SendAt: s.SendAt, CreatedAt: s.CreatedAt}
}

type scheduleIn struct {
	Body   string    `json:"body"`
	SendAt time.Time `json:"send_at"`
}

func (h *Handler) schedule(w http.ResponseWriter, r *http.Request) {
	var in scheduleIn
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	sc, err := h.svc.Schedule(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Body, in.SendAt)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toScheduled(sc))
}

func (h *Handler) scheduledList(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ScheduledFor(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]scheduledDTO, len(list))
	for i, sc := range list {
		out[i] = toScheduled(sc)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) reschedule(w http.ResponseWriter, r *http.Request) {
	var in scheduleIn
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	sc, err := h.svc.Reschedule(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Body, in.SendAt)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toScheduled(sc))
}

func (h *Handler) cancelScheduled(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.CancelScheduled(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// remind: {"text", "at"} — /remind me.
func (h *Handler) remind(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string    `json:"text"`
		At   time.Time `json:"at"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	rem, err := h.svc.Remind(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Text, in.At)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": string(rem.ID), "text": rem.Text, "at": rem.At})
}

func (h *Handler) receipts(w http.ResponseWriter, r *http.Request) {
	got, err := h.svc.Receipts(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type receipt struct {
		UserID string    `json:"user_id"`
		ReadAt time.Time `json:"read_at"`
	}
	out := []receipt{}
	for u, at := range got {
		out = append(out, receipt{UserID: u, ReadAt: at})
	}
	slices.SortFunc(out, func(a, b receipt) int { return strings.Compare(a.UserID, b.UserID) })
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// managed answers with the channel (as the actor sees it) after a management action.
func (h *Handler) managed(w http.ResponseWriter, r *http.Request, _ *domain.Conversation, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.get(w, r)
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string `json:"name"`
		Topic string `json:"topic"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Rename(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Name, in.Topic)
	h.managed(w, r, c, err)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Archive(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	h.managed(w, r, c, err)
}

func (h *Handler) unarchive(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Unarchive(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	h.managed(w, r, c, err)
}

func (h *Handler) makeAdmin(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.MakeAdmin(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.PathValue("user"))
	h.managed(w, r, c, err)
}

func (h *Handler) removeAdmin(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.RemoveAdmin(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.PathValue("user"))
	h.managed(w, r, c, err)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.RemoveMember(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), r.PathValue("user"))
	h.managed(w, r, c, err)
}

type groupDTO struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	MemberIDs []string  `json:"member_ids"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toGroup(g *domain.Group) groupDTO {
	d := groupDTO{ID: string(g.ID), Handle: g.Handle, Name: g.Name, MemberIDs: []string{}, CreatedBy: string(g.CreatedBy), CreatedAt: g.CreatedAt}
	for _, m := range g.Members {
		d.MemberIDs = append(d.MemberIDs, string(m))
	}
	return d
}

type groupIn struct {
	Handle    string   `json:"handle"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
}

func (h *Handler) groups(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Groups(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]groupDTO, len(list))
	for i, g := range list {
		out[i] = toGroup(g)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var in groupIn
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	g, err := h.svc.CreateGroup(r.Context(), auth.Actor(r.Context()), in.Handle, in.Name, in.MemberIDs)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toGroup(g))
}

func (h *Handler) updateGroup(w http.ResponseWriter, r *http.Request) {
	var in groupIn
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	g, err := h.svc.UpdateGroup(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Handle, in.Name, in.MemberIDs)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toGroup(g))
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteGroup(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type prefDTO struct {
	ConversationID string `json:"conversation_id"`
	Level          string `json:"level"`
	Section        string `json:"section"`
}

func toPref(p *domain.Pref) prefDTO {
	return prefDTO{ConversationID: string(p.Conversation), Level: string(p.Level), Section: p.Section}
}

func (h *Handler) setPref(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Level   string `json:"level"`
		Section string `json:"section"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SetPref(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), domain.Level(in.Level), in.Section)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toPref(p))
}

func (h *Handler) prefs(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.PrefsFor(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]prefDTO, len(list))
	for i, p := range list {
		out[i] = toPref(p)
	}
	slices.SortFunc(out, func(a, b prefDTO) int { return strings.Compare(a.ConversationID, b.ConversationID) })
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// linkProject: {"project_key": "KYB"} posts the project's news here; "" unlinks (S73).
func (h *Handler) linkProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Project string `json:"project_key"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.LinkProject(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Project)
	h.managed(w, r, c, err)
}

type bookmarkDTO struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Ref     string    `json:"ref"`
	Title   string    `json:"title"`
	AddedBy string    `json:"added_by"`
	AddedAt time.Time `json:"added_at"`
}

func toBookmark(b *domain.Bookmark) bookmarkDTO {
	return bookmarkDTO{ID: string(b.ID), Kind: string(b.Kind), Ref: b.Ref, Title: b.Title, AddedBy: string(b.AddedBy), AddedAt: b.AddedAt}
}

func (h *Handler) bookmarks(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Bookmarks(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]bookmarkDTO, len(list))
	for i, b := range list {
		out[i] = toBookmark(b)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) addBookmark(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind  string `json:"kind"`
		Ref   string `json:"ref"`
		Title string `json:"title"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	b, err := h.svc.AddBookmark(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), domain.BookmarkKind(in.Kind), in.Ref, in.Title)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toBookmark(b))
}

func (h *Handler) removeBookmark(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveBookmark(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type workflowDTO struct {
	ID        string    `json:"id"`
	Trigger   string    `json:"trigger"`
	Minute    int       `json:"minute"`
	Days      []int     `json:"days"`
	Text      string    `json:"text"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toWorkflow(w *domain.Workflow) workflowDTO {
	d := workflowDTO{ID: string(w.ID), Trigger: string(w.Trigger), Minute: w.Minute, Days: []int{}, Text: w.Text, CreatedBy: string(w.CreatedBy), CreatedAt: w.CreatedAt}
	for day := time.Sunday; day <= time.Saturday; day++ {
		if w.Days.Has(day) {
			d.Days = append(d.Days, int(day))
		}
	}
	return d
}

func (h *Handler) workflows(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Workflows(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]workflowDTO, len(list))
	for i, x := range list {
		out[i] = toWorkflow(x)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// addWorkflow: {"trigger": "member_joined"|"schedule", "minute": 0–1439 (UTC), "days": [0–6, Sunday = 0], "text"}.
func (h *Handler) addWorkflow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Trigger string `json:"trigger"`
		Minute  int    `json:"minute"`
		Days    []int  `json:"days"`
		Text    string `json:"text"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	var days domain.Days
	for _, d := range in.Days {
		if d < 0 || d > 6 {
			h.fail(w, r, domain.ErrInvalidWorkflow)
			return
		}
		days |= 1 << uint(d)
	}
	wf, err := h.svc.AddWorkflow(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), domain.Trigger(in.Trigger), in.Minute, days, in.Text)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toWorkflow(wf))
}

func (h *Handler) removeWorkflow(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RemoveWorkflow(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
