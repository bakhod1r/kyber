package httpapi

import (
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type historyDTO struct {
	At       time.Time `json:"at"`
	ByID     *string   `json:"by_id"` // null for system actions
	ByName   string    `json:"by_name"`
	Field    string    `json:"field"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	FromName string    `json:"from_name"`
	ToName   string    `json:"to_name"`
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	lines, err := h.svc.History(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]historyDTO, 0, len(lines))
	for _, l := range lines {
		d := historyDTO{At: l.At.UTC(), ByName: l.ByName, Field: l.Field, From: l.From, To: l.To, FromName: l.FromName, ToName: l.ToName}
		if l.By != "" {
			by := string(l.By)
			d.ByID = &by
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}
