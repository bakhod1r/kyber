package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

// ErrNoMailer is what an InvitationMailer returns when the server sends no email; the admin shares the link.
var ErrNoMailer = errors.New("email is not configured")

// InvitationMailer is the anti-corruption port to the Notifications context's email queue.
type InvitationMailer interface {
	// key (the invitation id) makes a repeated send idempotent.
	SendInvitation(ctx context.Context, workspace, to, subject, body, key string) error
}

// InvitationDeps wires invitations; NewToken returns an unguessable link secret.
type InvitationDeps struct {
	Repo      domain.InvitationRepository
	Mailer    InvitationMailer
	PublicURL string
	NewToken  func() string
	Now       func() time.Time
}

// WithInvitations turns on inviting people by email.
func (s *Service) WithInvitations(d InvitationDeps) *Service { s.inv = d; return s }

// InviteResult says whether an existing user was added straight away or an invitation was sent.
type InviteResult struct {
	Added      bool
	Invitation *domain.Invitation
	Link       string // the accept link (for the admin to share when Emailed is false)
	Emailed    bool
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Invite adds someone to the project by email; admin only. Someone with an account is added right away,
// anyone else gets an email with a link that works for InvitationTTL.
func (s *Service) Invite(ctx context.Context, actor domain.UserID, key, email, rawRole string) (InviteResult, error) {
	role, err := domain.ParseRole(rawRole)
	if err != nil {
		return InviteResult{}, err
	}
	p, err := s.byKey(ctx, key)
	if err != nil {
		return InviteResult{}, err
	}
	if err := p.Authorize(actor, domain.PermAdmin); err != nil {
		return InviteResult{}, err
	}
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return InviteResult{}, err
	}
	if _, err := s.users.ByEmail(ctx, normalized); err == nil {
		return InviteResult{Added: true}, s.SetMember(ctx, actor, key, normalized, rawRole)
	} else if !errors.Is(err, ErrUnknownUser) {
		return InviteResult{}, err
	}
	token := s.inv.NewToken()
	inv, err := domain.NewInvitation(domain.InvitationID(s.newID()), p.Workspace(), p.Key(), email, role, actor, hashToken(token), s.inv.Now())
	if err != nil {
		return InviteResult{}, err
	}
	if err := s.inv.Repo.Save(ctx, inv); err != nil {
		return InviteResult{}, err
	}
	res := InviteResult{Invitation: inv, Link: strings.TrimRight(s.inv.PublicURL, "/") + "/invite/" + token}
	inviter := string(actor)
	if u, err := s.users.ByID(ctx, actor); err == nil && u.Name != "" {
		inviter = u.Name
	}
	subject := fmt.Sprintf("%s invited you to %s on Kyber", inviter, p.Name())
	body := fmt.Sprintf("%s invited you to join the project %s (%s) as %s.\n\nAccept the invitation: %s\n\n"+
		"Sign in or sign up with %s to accept. The link works until %s.\n",
		inviter, p.Name(), p.Key(), role, res.Link, inv.Email(), inv.ExpiresAt().Format("2 Jan 2006 15:04 MST"))
	if s.inv.Mailer != nil {
		switch err := s.inv.Mailer.SendInvitation(ctx, string(p.Workspace()), inv.Email(), subject, body, "invite/"+string(inv.ID())); {
		case err == nil:
			res.Emailed = true
		case !errors.Is(err, ErrNoMailer):
			return res, err
		}
	}
	return res, nil
}

// Invitations lists the project's pending invitations; admin only.
func (s *Service) Invitations(ctx context.Context, actor domain.UserID, key string) ([]*domain.Invitation, error) {
	if err := s.Authorize(ctx, actor, key, domain.PermAdmin); err != nil {
		return nil, err
	}
	return s.inv.Repo.Pending(ctx, key)
}

// RevokeInvitation cancels a pending invitation of the project; admin only.
func (s *Service) RevokeInvitation(ctx context.Context, actor domain.UserID, key, id string) error {
	if err := s.Authorize(ctx, actor, key, domain.PermAdmin); err != nil {
		return err
	}
	inv, err := s.inv.Repo.ByID(ctx, domain.InvitationID(id))
	if err != nil {
		return err
	}
	if inv.Project() != key {
		return domain.ErrInvitationNotFound
	}
	return s.inv.Repo.Delete(ctx, inv.ID())
}

// InvitationPreview is what the holder of a link sees before accepting.
type InvitationPreview struct {
	Project, ProjectName, InvitedBy, Email string
	Role                                   domain.Role
	ExpiresAt                              time.Time
	Expired                                bool
}

func (s *Service) pendingByToken(ctx context.Context, token string) (*domain.Invitation, context.Context, error) {
	inv, err := s.inv.Repo.ByTokenHash(ctx, hashToken(token))
	if err != nil {
		return nil, ctx, err
	}
	return inv, tenant.With(ctx, string(inv.Workspace())), nil
}

// InvitationByToken shows an invitation to whoever holds its link (they may not have an account yet).
func (s *Service) InvitationByToken(ctx context.Context, token string) (InvitationPreview, error) {
	inv, wctx, err := s.pendingByToken(ctx, token)
	if err != nil {
		return InvitationPreview{}, err
	}
	if inv.AcceptedBy() != "" {
		return InvitationPreview{}, domain.ErrInvitationUsed
	}
	p, err := s.repo.ByKey(wctx, inv.Project())
	if err != nil {
		return InvitationPreview{}, err
	}
	by := string(inv.InvitedBy())
	if u, err := s.users.ByID(ctx, inv.InvitedBy()); err == nil && u.Name != "" {
		by = u.Name
	}
	return InvitationPreview{Project: p.Key(), ProjectName: p.Name(), InvitedBy: by, Email: inv.Email(), Role: inv.Role(),
		ExpiresAt: inv.ExpiresAt(), Expired: !inv.Pending(s.inv.Now())}, nil
}

// AcceptInvitation makes the signed-in user a member of the invitation's project (and its workspace), if
// they are signed in with the invited address. An existing member keeps their role. Returns the project key.
func (s *Service) AcceptInvitation(ctx context.Context, user domain.UserID, token string) (string, error) {
	inv, wctx, err := s.pendingByToken(ctx, token)
	if err != nil {
		return "", err
	}
	u, err := s.users.ByID(ctx, user)
	if err != nil {
		return "", err
	}
	if err := inv.Accept(user, u.Email, s.inv.Now()); err != nil {
		return "", err
	}
	err = s.repo.Update(wctx, inv.Project(), func(p *domain.Project) error {
		if _, ok := p.RoleOf(user); ok {
			return nil
		}
		return p.SetMember(user, inv.Role())
	})
	if err != nil {
		return "", err
	}
	if s.workspaces != nil {
		if err := s.workspaces.Join(wctx, string(inv.Workspace()), string(user)); err != nil {
			return "", err
		}
	}
	return inv.Project(), s.inv.Repo.Save(wctx, inv)
}
