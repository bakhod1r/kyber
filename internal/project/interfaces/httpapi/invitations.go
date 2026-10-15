package httpapi

import (
	"net/http"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/httpx"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

type invitationDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	InvitedBy string    `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func toInvitationDTO(i *domain.Invitation) invitationDTO {
	return invitationDTO{ID: string(i.ID()), Email: i.Email(), Role: string(i.Role()), InvitedBy: string(i.InvitedBy()),
		CreatedAt: i.CreatedAt(), ExpiresAt: i.ExpiresAt()}
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Role string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	res, err := h.svc.Invite(r.Context(), actor(r), r.PathValue("key"), in.Email, in.Role)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	if res.Added {
		httpx.JSON(w, http.StatusOK, map[string]any{"added": true})
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"added": false, "invitation": toInvitationDTO(res.Invitation),
		"link": res.Link, "emailed": res.Emailed})
}

func (h *Handler) invitations(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Invitations(r.Context(), actor(r), r.PathValue("key"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	items := make([]invitationDTO, 0, len(list))
	for _, i := range list {
		items = append(items, toInvitationDTO(i))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeInvitation(r.Context(), actor(r), r.PathValue("key"), r.PathValue("id")); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) previewInvitation(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.InvitationByToken(r.Context(), r.PathValue("token"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"project": p.Project, "project_name": p.ProjectName, "invited_by": p.InvitedBy,
		"email": p.Email, "role": string(p.Role), "expires_at": p.ExpiresAt, "expired": p.Expired})
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	key, err := h.svc.AcceptInvitation(r.Context(), actor(r), r.PathValue("token"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"project": key})
}
