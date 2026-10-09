// Package httpapi exposes the workspace time table (meetings) over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/app"
	"github.com/bakhod1r/kyber/internal/calendar/domain"
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
	mux.HandleFunc("GET /api/v1/meetings", h.list)
	mux.HandleFunc("POST /api/v1/meetings", h.create)
	mux.HandleFunc("DELETE /api/v1/meetings/{id}", h.cancel)
}

var errBadRange = errors.New("from and to must be RFC 3339 times")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrMeetingNotFound):
		return httpx.CodeMeetingNotFound, true
	case errors.Is(err, app.ErrForbidden):
		return httpx.CodeMeetingForbidden, true
	case errors.Is(err, domain.ErrEmptyTitle), errors.Is(err, domain.ErrInvalidTime), errors.Is(err, app.ErrNotMember),
		errors.Is(err, app.ErrRange), errors.Is(err, errBadRange), errors.Is(err, app.ErrInPast):
		return httpx.CodeValidation, true
	case errors.Is(err, httpx.ErrBadJSON):
		return httpx.CodeBadRequest, true
	}
	return "", false
}

type meetingDTO struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	OrganizerID string    `json:"organizer_id"`
	AttendeeIDs []string  `json:"attendee_ids"`
}

func toDTO(m *domain.Meeting) meetingDTO {
	d := meetingDTO{ID: string(m.ID), Title: m.Title, StartsAt: m.Start, EndsAt: m.End, OrganizerID: string(m.Organizer), AttendeeIDs: []string{}}
	for _, a := range m.Attendees {
		d.AttendeeIDs = append(d.AttendeeIDs, string(a))
	}
	return d
}

// list: ?from=&to= (RFC 3339); default is today (UTC) and the next 7 days.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	day := h.now().UTC().Truncate(24 * time.Hour)
	from, to := day, day.Add(7*24*time.Hour)
	q := r.URL.Query()
	for _, p := range []struct {
		name string
		dst  *time.Time
	}{{"from", &from}, {"to", &to}} {
		if v := q.Get(p.name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				httpx.Error(w, r, h.log, errBadRange, codeFor)
				return
			}
			*p.dst = t
		}
	}
	ms, err := h.svc.Timetable(r.Context(), auth.Actor(r.Context()), from, to)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]meetingDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, toDTO(m))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title       string    `json:"title"`
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
		AttendeeIDs []string  `json:"attendee_ids"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	m, err := h.svc.Schedule(r.Context(), auth.Actor(r.Context()), app.Schedule{Title: in.Title, Start: in.StartsAt, End: in.EndsAt, Attendees: in.AttendeeIDs})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(m))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Cancel(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
