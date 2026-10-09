// Package httpapi exposes the Insights reports over REST.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/app"
	"github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type Handler struct {
	svc *app.Service
	log *slog.Logger
}

func New(svc *app.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{key}/reports/summary", h.summary)
	mux.HandleFunc("GET /api/v1/projects/{key}/reports/created-vs-resolved", h.createdVsResolved)
	mux.HandleFunc("GET /api/v1/projects/{key}/reports/velocity", h.velocity)
	mux.HandleFunc("GET /api/v1/sprints/{id}/burndown", h.burndown)
}

var errBadDays = errors.New("days must be an integer between 1 and 365")

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, app.ErrProjectNotFound):
		return httpx.CodeProjectNotFound, true
	case errors.Is(err, app.ErrSprintNotFound):
		return httpx.CodeSprintNotFound, true
	case errors.Is(err, errBadDays):
		return httpx.CodeValidation, true
	}
	return "", false
}

type countDTO struct {
	Key    string  `json:"key"`
	Count  int     `json:"count"`
	Points float64 `json:"points"`
}

func counts(cs []app.Count) []countDTO {
	out := make([]countDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, countDTO{Key: c.Key, Count: c.Count, Points: domain.Tenths(c.Points)})
	}
	return out
}

func hours(d time.Duration) float64 { return float64(d.Round(time.Minute)) / float64(time.Hour) }

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Summary(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	type loadDTO struct {
		UserID *string `json:"user_id"`
		Name   string  `json:"name"`
		Count  int     `json:"count"`
		Points float64 `json:"points"`
	}
	loads := make([]loadDTO, 0, len(s.Workload))
	for _, l := range s.Workload {
		d := loadDTO{Name: l.Name, Count: l.Count, Points: domain.Tenths(l.Points)}
		if l.UserID != "" {
			id := l.UserID
			d.UserID = &id
		}
		loads = append(loads, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"total": s.Total, "open": s.Open, "done": s.Done, "unassigned": s.Unassigned,
		"total_points": domain.Tenths(s.TotalPoints), "open_points": domain.Tenths(s.OpenPoints),
		"by_status": counts(s.ByStatus), "by_type": counts(s.ByType), "by_priority": counts(s.ByPriority),
		"workload":   loads,
		"cycle_time": map[string]any{"count": s.Cycle.Count, "average_hours": hours(s.Cycle.Average), "median_hours": hours(s.Cycle.Median)},
	})
}

func (h *Handler) createdVsResolved(w http.ResponseWriter, r *http.Request) {
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 365 {
			httpx.Error(w, r, h.log, errBadDays, codeFor)
			return
		}
		days = n
	}
	list, err := h.svc.CreatedVsResolved(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), days)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	type dayDTO struct {
		Date        string `json:"date"`
		Created     int    `json:"created"`
		Resolved    int    `json:"resolved"`
		CumCreated  int    `json:"cum_created"`
		CumResolved int    `json:"cum_resolved"`
	}
	out := make([]dayDTO, 0, len(list))
	for _, d := range list {
		out = append(out, dayDTO{Date: d.Date.Format(time.DateOnly), Created: d.Created, Resolved: d.Resolved, CumCreated: d.CumCreated, CumResolved: d.CumResolved})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"days": out})
}

type sampleDTO struct {
	At              time.Time `json:"at"`
	RemainingPoints float64   `json:"remaining_points"`
	RemainingIssues int       `json:"remaining_issues"`
}

func samples(ss []domain.Sample) []sampleDTO {
	out := make([]sampleDTO, 0, len(ss))
	for _, s := range ss {
		out = append(out, sampleDTO{At: s.At.UTC(), RemainingPoints: domain.Tenths(s.RemainingPoints), RemainingIssues: s.RemainingIssues})
	}
	return out
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

func (h *Handler) burndown(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Burndown(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	rep := b.Report
	httpx.JSON(w, http.StatusOK, map[string]any{
		"sprint": map[string]any{"id": b.Sprint.ID, "name": b.Sprint.Name, "state": b.Sprint.State,
			"started_at": timePtr(b.Sprint.Start), "ends_at": timePtr(b.Sprint.End), "completed_at": timePtr(b.Sprint.Completed)},
		"start_points": domain.Tenths(rep.StartPoints), "start_issues": rep.StartIssues,
		"added_points": domain.Tenths(rep.AddedPoints), "added_issues": rep.AddedIssues, "removed_issues": rep.RemovedIssues,
		"samples": samples(rep.Samples), "ideal": samples(rep.Ideal),
	})
}

func (h *Handler) velocity(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Velocity(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	type vDTO struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		CommittedPoints float64 `json:"committed_points"`
		CompletedPoints float64 `json:"completed_points"`
		CompletedIssues int     `json:"completed_issues"`
	}
	out := make([]vDTO, 0, len(list))
	for _, v := range list {
		out = append(out, vDTO{ID: v.ID, Name: v.Name, CommittedPoints: domain.Tenths(v.CommittedPoints),
			CompletedPoints: domain.Tenths(v.CompletedPoints), CompletedIssues: v.CompletedIssues})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sprints": out})
}
