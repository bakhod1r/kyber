// Package httpapi exposes the caller's notifications over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notifications", h.list)
	mux.HandleFunc("POST /api/v1/notifications/read-all", h.readAll)
	mux.HandleFunc("POST /api/v1/notifications/{id}/read", h.read)
	mux.HandleFunc("GET /api/v1/me/notification-settings", h.settings)
	mux.HandleFunc("PUT /api/v1/me/notification-settings", h.setSettings)
	mux.HandleFunc("POST /api/v1/projects/{key}/telegram-groups/link", h.groupLink)
	mux.HandleFunc("GET /api/v1/projects/{key}/telegram-groups", h.groups)
	mux.HandleFunc("DELETE /api/v1/projects/{key}/telegram-groups/{chat}", h.unlinkGroup)
}

func (h *Handler) groupLink(w http.ResponseWriter, r *http.Request) {
	link, expires, err := h.svc.StartGroupLink(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"url": link, "expires_at": expires.UTC()})
}

type groupDTO struct {
	ChatID   string    `json:"chat_id"`
	Title    string    `json:"title"`
	LinkedAt time.Time `json:"linked_at"`
}

func (h *Handler) groups(w http.ResponseWriter, r *http.Request) {
	chats, err := h.svc.GroupChats(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]groupDTO, 0, len(chats))
	for _, c := range chats {
		out = append(out, groupDTO{ChatID: c.ChatID, Title: c.Title, LinkedAt: c.LinkedAt.UTC()})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) unlinkGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.UnlinkGroup(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), r.PathValue("chat")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type settingsDTO struct {
	Email             bool   `json:"email"`
	Telegram          bool   `json:"telegram"`
	EmailAvailable    bool   `json:"email_available"`
	TelegramAvailable bool   `json:"telegram_available"`
	TelegramConnected bool   `json:"telegram_connected"`
	TimeZone          string `json:"time_zone"`
	EmailDigest       bool   `json:"email_digest"`
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Settings(r.Context(), auth.Actor(r.Context()))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, settingsDTO{Email: s.Email, Telegram: s.Telegram, EmailAvailable: s.EmailAvailable,
		TelegramAvailable: s.TelegramAvailable, TelegramConnected: s.TelegramConnected, TimeZone: s.TimeZone, EmailDigest: s.EmailDigest})
}

func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    *bool   `json:"email"`
		Telegram *bool   `json:"telegram"`
		TimeZone *string `json:"time_zone"`
		Digest   *bool   `json:"email_digest"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	cur, err := h.svc.Settings(r.Context(), actor)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	p := cur.Prefs
	if in.Email != nil {
		p.Email = *in.Email
	}
	if in.Telegram != nil {
		p.Telegram = *in.Telegram
	}
	if in.TimeZone != nil {
		p.TimeZone = *in.TimeZone
	}
	if in.Digest != nil {
		p.EmailDigest = *in.Digest
	}
	if err := h.svc.SetPrefs(r.Context(), actor, p); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	h.settings(w, r)
}

type notificationDTO struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	IssueKey   string    `json:"issue_key"`
	IssueTitle string    `json:"issue_title"`
	ConvID     string    `json:"conversation_id"`
	ConvName   string    `json:"conversation_name"`
	PageID     string    `json:"page_id,omitempty"` // S95: the commented doc page
	ProjectKey string    `json:"project_key"`
	ActorName  string    `json:"actor_name"`
	Excerpt    string    `json:"excerpt"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrNotificationNotFound):
		return httpx.CodeNotificationNotFound, true
	case errors.Is(err, application.ErrNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, application.ErrForbidden):
		return httpx.CodeForbidden, true
	case errors.Is(err, application.ErrGroupsDisabled):
		return httpx.CodeProviderDisabled, true
	case errors.Is(err, domain.ErrInvalidTimeZone):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrChatNotLinked):
		return httpx.CodeNotFound, true
	}
	return "", false
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, unread, err := h.svc.List(r.Context(), auth.Actor(r.Context()), r.URL.Query().Get("unread") == "true")
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]notificationDTO, 0, len(items))
	for _, n := range items {
		out = append(out, notificationDTO{ID: string(n.ID), Kind: string(n.Kind), IssueKey: n.IssueKey, IssueTitle: n.IssueTitle,
			ConvID: n.ConversationID, ConvName: n.ConversationName, PageID: n.PageID,
			ProjectKey: n.Project, ActorName: n.ActorName, Excerpt: n.Excerpt, Read: n.Read(), CreatedAt: n.CreatedAt.UTC()})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out, "unread": unread})
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkRead(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	read := func() error { return h.svc.MarkAllRead(r.Context(), auth.Actor(r.Context())) }
	if conv := r.URL.Query().Get("conversation_id"); conv != "" {
		read = func() error { return h.svc.MarkConversationRead(r.Context(), auth.Actor(r.Context()), conv) }
	}
	if err := read(); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
