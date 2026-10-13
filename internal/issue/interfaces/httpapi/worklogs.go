package httpapi

import (
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// timeDTO is an issue's time tracking in seconds, with Jira-style text ("1d 4h").
type timeDTO struct {
	OriginalSeconds  int64  `json:"original_seconds"`
	RemainingSeconds int64  `json:"remaining_seconds"`
	SpentSeconds     int64  `json:"spent_seconds"`
	Original         string `json:"original"`
	Remaining        string `json:"remaining"`
	Spent            string `json:"spent"`
}

func text(s domain.Seconds) string {
	if s == 0 {
		return ""
	}
	return s.String()
}

func timeOut(t domain.TimeTracking) timeDTO {
	return timeDTO{OriginalSeconds: int64(t.Original), RemainingSeconds: int64(t.Remaining), SpentSeconds: int64(t.Spent),
		Original: text(t.Original), Remaining: text(t.Remaining), Spent: text(t.Spent)}
}

type worklogDTO struct {
	ID         string    `json:"id"`
	IssueKey   string    `json:"issue_key"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Started    time.Time `json:"started"`
	Seconds    int64     `json:"seconds"`
	Spent      string    `json:"spent"`
	Comment    string    `json:"comment"`
}

func worklogOut(w domain.Worklog, name string) worklogDTO {
	return worklogDTO{ID: string(w.ID), IssueKey: w.Key.String(), AuthorID: string(w.Author), AuthorName: name,
		Started: w.Started.UTC(), Seconds: int64(w.Seconds), Spent: w.Seconds.String(), Comment: w.Comment}
}

func (h *Handler) worklogs(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Worklogs(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]worklogDTO, 0, len(list))
	for _, x := range list {
		out = append(out, worklogOut(x.Worklog, x.AuthorName))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) logWork(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Spent   string    `json:"spent"`
		Started time.Time `json:"started"`
		Comment string    `json:"comment"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor := auth.Actor(r.Context())
	wl, err := h.svc.LogWork(r.Context(), actor, r.PathValue("issueKey"), application.LogWork{Spent: in.Spent, Started: in.Started, Comment: in.Comment})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, worklogOut(wl, ""))
}

func (h *Handler) deleteWorklog(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteWorklog(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
