package httpapi

import (
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/application"
	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type versionDTO struct {
	ID          string     `json:"id"`
	ProjectKey  string     `json:"project_key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ReleaseDate *time.Time `json:"release_date"`
	State       string     `json:"state"`
	ReleasedAt  *time.Time `json:"released_at"`
	Version     int        `json:"version"`
	Issues      *int       `json:"issues,omitempty"`
	Done        *int       `json:"done,omitempty"`
	Unresolved  *int       `json:"unresolved,omitempty"`
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

func versionOut(v *domain.Version) versionDTO {
	return versionDTO{ID: string(v.ID()), ProjectKey: v.Project(), Name: v.Name(), Description: v.Description(),
		ReleaseDate: timeOrNil(v.ReleaseDate()), State: string(v.State()), ReleasedAt: timeOrNil(v.ReleasedAt()), Version: v.Version()}
}

type versionIn struct {
	Version     int        `json:"version"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ReleaseDate *time.Time `json:"release_date"`
}

func (in versionIn) spec() application.VersionSpec {
	return application.VersionSpec{Name: in.Name, Description: in.Description, ReleaseDate: in.ReleaseDate}
}

func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Versions(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]versionDTO, 0, len(list))
	for _, v := range list {
		d := versionOut(v.Version)
		d.Issues, d.Done, d.Unresolved = &v.Issues, &v.Done, &v.Unresolved
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createVersion(w http.ResponseWriter, r *http.Request) {
	var in versionIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	v, err := h.svc.CreateVersion(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, versionOut(v))
}

func (h *Handler) editVersion(w http.ResponseWriter, r *http.Request) {
	var in versionIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	v, err := h.svc.EditVersion(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Version, in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, versionOut(v))
}

// moveTo reads an optional {"move_to": "<version id>"} body.
func moveTo(r *http.Request) (string, error) {
	var in struct {
		MoveTo string `json:"move_to"`
	}
	if r.ContentLength == 0 {
		return "", nil
	}
	err := httpx.Decode(r, &in)
	return in.MoveTo, err
}

func (h *Handler) releaseVersion(w http.ResponseWriter, r *http.Request) {
	to, err := moveTo(r)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	v, moved, err := h.svc.ReleaseVersion(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), to)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"version": versionOut(v), "moved": moved})
}

func (h *Handler) versionStep(step func(*domain.Version) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := h.svc.ChangeVersionState(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), step)
		if err != nil {
			httpx.Error(w, r, h.log, err, codeFor)
			return
		}
		httpx.JSON(w, http.StatusOK, versionOut(v))
	}
}

func (h *Handler) deleteVersion(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("move_to")
	if err := h.svc.DeleteVersion(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), to); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type versionIssueDTO struct {
	Key        string `json:"key"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Done       bool   `json:"done"`
	Resolution string `json:"resolution"`
}

func (h *Handler) versionIssues(w http.ResponseWriter, r *http.Request) {
	v, issues, err := h.svc.VersionIssues(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]versionIssueDTO, 0, len(issues))
	for _, is := range issues {
		out = append(out, versionIssueDTO{Key: is.Key, Title: is.Title, Type: is.Type, Done: is.Done, Resolution: is.Resolution})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"version": versionOut(v), "items": out})
}

func (h *Handler) releaseNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := h.svc.ReleaseNotes(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write([]byte(notes))
}
