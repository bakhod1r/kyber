package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type linkDTO struct {
	ID     string   `json:"id"`
	Type   string   `json:"type"`
	Phrase string   `json:"phrase"` // read from the requested issue's side
	Issue  issueDTO `json:"issue"`  // the issue at the other end
}

func toLinkDTO(v app.LinkView) linkDTO {
	return linkDTO{ID: string(v.ID), Type: string(v.Type), Phrase: v.Phrase, Issue: toDTO(v.Other)}
}

func (h *Handler) links(w http.ResponseWriter, r *http.Request) {
	views, err := h.svc.Links(r.Context(), auth.Actor(r.Context()), r.PathValue("issueKey"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]linkDTO, 0, len(views))
	for _, v := range views {
		out = append(out, toLinkDTO(v))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) link(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Type  string `json:"type"`
		ToKey string `json:"to_key"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	actor, from := auth.Actor(r.Context()), r.PathValue("issueKey")
	l, err := h.svc.Link(r.Context(), actor, from, in.Type, in.ToKey)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	other, err := h.svc.Get(r.Context(), actor, in.ToKey)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toLinkDTO(app.LinkView{Link: l, Phrase: l.PhraseFrom(l.From), Other: other}))
}

func (h *Handler) unlink(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unlink(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
