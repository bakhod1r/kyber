// Package httpapi exposes Pomodoro focus sessions and their history over REST.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/bakhod1r/errorx"

	"github.com/bakhod1r/kyber/internal/focus/app"
	"github.com/bakhod1r/kyber/internal/focus/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
	now func() time.Time
}

func New(svc *app.Service, log *slog.Logger, now func() time.Time) *Handler {
	return &Handler{svc: svc, log: log, now: now}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/focus", h.current)
	mux.HandleFunc("POST /api/v1/focus", h.start)
	mux.HandleFunc("POST /api/v1/focus/pause", h.action((*app.Service).Pause))
	mux.HandleFunc("POST /api/v1/focus/resume", h.action((*app.Service).Resume))
	mux.HandleFunc("POST /api/v1/focus/stop", h.action((*app.Service).Stop))
	mux.HandleFunc("GET /api/v1/issues/{issueKey}/focus", h.issueLog)
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrMeetingConflict):
		return httpx.CodeFocusMeeting, true
	case errors.Is(err, domain.ErrAlreadyRunning):
		return httpx.CodeFocusRunning, true
	case errors.Is(err, domain.ErrSessionNotFound):
		return httpx.CodeFocusNotFound, true
	case errors.Is(err, domain.ErrNotRunning), errors.Is(err, domain.ErrNotPaused), errors.Is(err, domain.ErrFinished):
		return httpx.CodeFocusState, true
	case errors.Is(err, app.ErrIssueNotFound):
		return httpx.CodeIssueNotFound, true
	case errors.Is(err, domain.ErrInvalidLength), errors.Is(err, domain.ErrNoIssue):
		return httpx.CodeValidation, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

// fail explains a meeting conflict with the meeting's name and time.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var c *domain.MeetingConflictError
	if errors.As(err, &c) {
		httpx.Problem(w, r, errorx.New(httpx.CodeFocusMeeting, err.Error()).WithDetails(err.Error()))
		return
	}
	httpx.Error(w, r, h.log, err, codeFor)
}

type sessionDTO struct {
	ID               string     `json:"id"`
	IssueKey         string     `json:"issue_key"`
	UserID           string     `json:"user_id"`
	State            string     `json:"state"`
	PlannedMinutes   int        `json:"planned_minutes"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	FocusedSeconds   int        `json:"focused_seconds"`
	RemainingSeconds int        `json:"remaining_seconds"`
	Reason           string     `json:"reason"`
}

func (h *Handler) dto(s *domain.Session) sessionDTO {
	focused := s.Focused(h.now())
	d := sessionDTO{ID: s.ID, IssueKey: s.Issue, UserID: s.User, State: string(s.State), PlannedMinutes: int(s.Planned.Minutes()),
		StartedAt: s.StartedAt, FocusedSeconds: int(math.Round(focused.Seconds())), Reason: s.Reason}
	if !s.EndedAt.IsZero() {
		e := s.EndedAt
		d.EndedAt = &e
	} else {
		d.RemainingSeconds = int(math.Round((s.Planned - focused).Seconds()))
	}
	return d
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Current(r.Context(), auth.Actor(r.Context()))
	if errors.Is(err, domain.ErrSessionNotFound) {
		httpx.JSON(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"session": h.dto(s)})
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	in := struct {
		IssueKey string `json:"issue_key"`
		Minutes  int    `json:"minutes"`
	}{Minutes: int(domain.DefaultFocus.Minutes())}
	if err := httpx.Decode(r, &in); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.Start(r.Context(), auth.Actor(r.Context()), in.IssueKey, in.Minutes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, h.dto(s))
}

func (h *Handler) action(fn func(*app.Service, context.Context, string) (*domain.Session, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := fn(h.svc, r.Context(), auth.Actor(r.Context()))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, h.dto(s))
	}
}

func (h *Handler) issueLog(w http.ResponseWriter, r *http.Request) {
	log, total, err := h.svc.IssueLog(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]sessionDTO, 0, len(log))
	for _, s := range log {
		items = append(items, h.dto(s))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total_focused_seconds": int(math.Round(total.Seconds()))})
}
