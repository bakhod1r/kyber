// Package httpapi exposes huddles (calls in a conversation) over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/call/application"
	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func New(svc *application.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/conversations/{id}/call", h.live)
	mux.HandleFunc("POST /api/v1/conversations/{id}/call", h.join)
	mux.HandleFunc("POST /api/v1/calls/{id}/leave", h.leave)
	mux.HandleFunc("PUT /api/v1/calls/{id}/media", h.media)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, application.ErrNoConversation):
		return httpx.CodeConversationNotFound, true
	case errors.Is(err, application.ErrNotMember):
		return httpx.CodeChatForbidden, true
	case errors.Is(err, domain.ErrCallNotFound):
		return httpx.CodeCallNotFound, true
	case errors.Is(err, domain.ErrCallFull):
		return httpx.CodeCallFull, true
	case errors.Is(err, domain.ErrNotInCall), errors.Is(err, domain.ErrCallEnded), errors.Is(err, domain.ErrConflict):
		return httpx.CodeCallState, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

// ParticipantDTO and CallDTO are also the shape of call.updated stream events.
type ParticipantDTO struct {
	UserID   string `json:"user_id"`
	JoinedAt string `json:"joined_at"`
	Audio    bool   `json:"audio"`
	Video    bool   `json:"video"`
	Screen   bool   `json:"screen"`
}

type CallDTO struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	StartedBy      string           `json:"started_by"`
	StartedAt      string           `json:"started_at"`
	Live           bool             `json:"live"`
	Participants   []ParticipantDTO `json:"participants"`
}

func ToDTO(c *domain.Call) CallDTO {
	d := CallDTO{ID: string(c.ID), ConversationID: string(c.Conversation), StartedBy: string(c.StartedBy),
		StartedAt: c.StartedAt.Format(time.RFC3339Nano), Live: c.Live(), Participants: []ParticipantDTO{}}
	for _, p := range c.Participants {
		d.Participants = append(d.Participants, ParticipantDTO{UserID: string(p.User), JoinedAt: p.JoinedAt.Format(time.RFC3339Nano),
			Audio: p.Media.Audio, Video: p.Media.Video, Screen: p.Media.Screen})
	}
	return d
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Error(w, r, h.log, err, codeFor)
}

func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Live(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var out *CallDTO
	if c != nil {
		d := ToDTO(c)
		out = &d
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"call": out})
}

func (h *Handler) join(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Join(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(c))
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Leave(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Audio  bool `json:"audio"`
		Video  bool `json:"video"`
		Screen bool `json:"screen"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.SetMedia(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), domain.Media{Audio: in.Audio, Video: in.Video, Screen: in.Screen})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(c))
}
