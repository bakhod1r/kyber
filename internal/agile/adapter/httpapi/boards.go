package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/agile/app"
	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

type boardDTO struct {
	ID         string          `json:"id"`
	ProjectKey string          `json:"project_key"`
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Columns    []domain.Column `json:"columns"`
	Version    int             `json:"version"`
}

func toBoardDTO(b *domain.Board) boardDTO {
	return boardDTO{ID: string(b.ID()), ProjectKey: b.Project(), Name: b.Name(), Kind: string(b.Kind()), Columns: b.Columns(), Version: b.Version()}
}

type boardIn struct {
	Name    string           `json:"name"`
	Kind    domain.BoardKind `json:"kind"`
	Columns []domain.Column  `json:"columns"`
	Version int              `json:"version"`
}

func (in boardIn) spec() app.BoardSpec {
	return app.BoardSpec{Name: in.Name, Kind: in.Kind, Columns: in.Columns}
}

func (h *Handler) boards(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Boards(r.Context(), auth.Actor(r.Context()), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	out := make([]boardDTO, 0, len(list))
	for _, b := range list {
		out = append(out, toBoardDTO(b))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createBoard(w http.ResponseWriter, r *http.Request) {
	var in boardIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	b, err := h.svc.CreateBoard(r.Context(), auth.Actor(r.Context()), r.PathValue("key"), in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toBoardDTO(b))
}

func (h *Handler) board(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Board(r.Context(), auth.Actor(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toBoardDTO(b))
}

func (h *Handler) updateBoard(w http.ResponseWriter, r *http.Request) {
	var in boardIn
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	b, err := h.svc.UpdateBoard(r.Context(), auth.Actor(r.Context()), r.PathValue("id"), in.Version, in.spec())
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, toBoardDTO(b))
}

func (h *Handler) deleteBoard(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteBoard(r.Context(), auth.Actor(r.Context()), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
