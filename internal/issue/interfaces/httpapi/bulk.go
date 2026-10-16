package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type bulkResultDTO struct {
	Key   string `json:"key"`
	OK    bool   `json:"ok"`
	Code  string `json:"code,omitempty"`  // the problem code, as a single request would return it
	Error string `json:"error,omitempty"` // why this issue was not changed
}

// bulk applies one change to several issues (G9); the response lists every issue's outcome.
func (h *Handler) bulk(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Keys      []string `json:"keys"`
		Status    *string  `json:"status"`
		Assignee  *string  `json:"assignee_id"`
		Sprint    *string  `json:"sprint_id"`
		AddLabels []string `json:"add_labels"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	res, err := h.svc.Bulk(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.Keys,
		application.BulkChange{Status: in.Status, Assignee: in.Assignee, Sprint: in.Sprint, AddLabels: in.AddLabels})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]bulkResultDTO, 0, len(res))
	for _, x := range res {
		d := bulkResultDTO{Key: x.Key, OK: x.Err == nil}
		if x.Err != nil {
			code, ok := codeFor(x.Err)
			if !ok {
				h.log.ErrorContext(r.Context(), "bulk change", "issue", x.Key, "err", x.Err)
				code, d.Error = httpx.CodeInternal, "internal error"
			} else {
				d.Error = x.Err.Error()
			}
			d.Code = code
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"results": out})
}
