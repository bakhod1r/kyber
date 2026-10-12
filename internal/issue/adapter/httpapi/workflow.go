package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

func workflowDTO(wf *domain.Workflow) domain.WorkflowDef {
	return domain.WorkflowDef{Initial: wf.Initial(), Statuses: wf.Statuses(), Transitions: append([]domain.Transition{}, wf.Transitions()...)}
}

func (h *Handler) workflow(w http.ResponseWriter, r *http.Request) {
	wf, err := h.svc.Workflow(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, workflowDTO(wf))
}

func (h *Handler) setWorkflow(w http.ResponseWriter, r *http.Request) {
	var in domain.WorkflowDef
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	wf, err := h.svc.SetWorkflow(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, workflowDTO(wf))
}
