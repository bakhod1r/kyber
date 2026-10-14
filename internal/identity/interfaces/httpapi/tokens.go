package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// errTokenScope: a request authenticated by a personal access token tried to manage tokens. A
// leaked token must not be able to mint successors or hide its revocation.
var errTokenScope = errors.New("personal access tokens cannot manage tokens")

type viaTokenKey struct{}

type tokenDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func tokenOut(t *domain.PersonalToken) tokenDTO {
	return tokenDTO{ID: string(t.ID), Name: t.Name, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt}
}

// sessionOnly rejects token management from personal-access-token requests.
func (h *Handler) sessionOnly(w http.ResponseWriter, r *http.Request) bool {
	if via, _ := r.Context().Value(viaTokenKey{}).(bool); via {
		httpx.Error(w, r, h.log, errTokenScope, codeFor)
		return false
	}
	return true
}

func (h *Handler) tokens(w http.ResponseWriter, r *http.Request) {
	if !h.sessionOnly(w, r) {
		return
	}
	list, err := h.svc.Tokens(r.Context(), domain.UserID(auth.Actor(r.Context())))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]tokenDTO, 0, len(list))
	for _, t := range list {
		items = append(items, tokenOut(t))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// createToken returns the secret once, next to the token's description.
func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	if !h.sessionOnly(w, r) {
		return
	}
	var in struct {
		Name string `json:"name"`
		Days int    `json:"expires_in_days"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	secret, t, err := h.svc.CreateToken(r.Context(), domain.UserID(auth.Actor(r.Context())), in.Name, in.Days)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, struct {
		tokenDTO
		Token string `json:"token"`
	}{tokenOut(t), secret})
}

func (h *Handler) revokeToken(w http.ResponseWriter, r *http.Request) {
	if !h.sessionOnly(w, r) {
		return
	}
	if err := h.svc.RevokeToken(r.Context(), domain.UserID(auth.Actor(r.Context())), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
