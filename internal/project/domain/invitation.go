package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bakhod1r/emailx"
)

// InvitationTTL is how long an invitation link works.
const InvitationTTL = 7 * 24 * time.Hour

var (
	ErrInvalidEmail             = errors.New("invalid email address")
	ErrInvitationNotFound       = errors.New("invitation not found")
	ErrInvitationExpired        = errors.New("invitation has expired")
	ErrInvitationUsed           = errors.New("invitation was already accepted")
	ErrInvitationForSomeoneElse = errors.New("invitation was sent to another email address")
)

type InvitationID string

// Invitation asks someone, by email, to join a project with a role. The link carries a secret token; only
// its hash is kept. Whoever accepts must be signed in with the invited address.
type Invitation struct {
	id         InvitationID
	workspace  WorkspaceID
	project    string
	email      string
	role       Role
	invitedBy  UserID
	tokenHash  string
	createdAt  time.Time
	expiresAt  time.Time
	acceptedBy UserID
	acceptedAt time.Time
}

func NewInvitation(id InvitationID, ws WorkspaceID, project, email string, role Role, by UserID, tokenHash string, now time.Time) (*Invitation, error) {
	e, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if _, err := ParseRole(string(role)); err != nil {
		return nil, err
	}
	now = now.UTC()
	return &Invitation{id: id, workspace: ws, project: project, email: e, role: role, invitedBy: by, tokenHash: tokenHash,
		createdAt: now, expiresAt: now.Add(InvitationTTL)}, nil
}

// NormalizeEmail trims and lower-cases an address and checks its syntax (as Identity does).
func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 254 || strings.ContainsAny(s, "<> \t\r\n") || !emailx.IsValid(s) {
		return "", ErrInvalidEmail
	}
	return s, nil
}

// InvitationSnapshot is the persisted state (repository use only).
type InvitationSnapshot struct {
	ID         InvitationID
	Workspace  WorkspaceID
	Project    string
	Email      string
	Role       Role
	InvitedBy  UserID
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AcceptedBy UserID
	AcceptedAt time.Time
}

func RehydrateInvitation(s InvitationSnapshot) *Invitation {
	return &Invitation{id: s.ID, workspace: s.Workspace, project: s.Project, email: s.Email, role: s.Role, invitedBy: s.InvitedBy,
		tokenHash: s.TokenHash, createdAt: s.CreatedAt, expiresAt: s.ExpiresAt, acceptedBy: s.AcceptedBy, acceptedAt: s.AcceptedAt}
}

func (i *Invitation) Snapshot() InvitationSnapshot {
	return InvitationSnapshot{ID: i.id, Workspace: i.workspace, Project: i.project, Email: i.email, Role: i.role, InvitedBy: i.invitedBy,
		TokenHash: i.tokenHash, CreatedAt: i.createdAt, ExpiresAt: i.expiresAt, AcceptedBy: i.acceptedBy, AcceptedAt: i.acceptedAt}
}

func (i *Invitation) ID() InvitationID       { return i.id }
func (i *Invitation) Workspace() WorkspaceID { return i.workspace }
func (i *Invitation) Project() string        { return i.project }
func (i *Invitation) Email() string          { return i.email }
func (i *Invitation) Role() Role             { return i.role }
func (i *Invitation) InvitedBy() UserID      { return i.invitedBy }
func (i *Invitation) TokenHash() string      { return i.tokenHash }
func (i *Invitation) CreatedAt() time.Time   { return i.createdAt }
func (i *Invitation) ExpiresAt() time.Time   { return i.expiresAt }
func (i *Invitation) AcceptedBy() UserID     { return i.acceptedBy }
func (i *Invitation) AcceptedAt() time.Time  { return i.acceptedAt }

// Pending tells whether the invitation can still be accepted at now.
func (i *Invitation) Pending(now time.Time) bool {
	return i.acceptedBy == "" && now.Before(i.expiresAt)
}

// Accept records that user, signed in as email, took the invitation.
func (i *Invitation) Accept(user UserID, email string, now time.Time) error {
	switch {
	case i.acceptedBy != "":
		return ErrInvitationUsed
	case !now.Before(i.expiresAt):
		return ErrInvitationExpired
	case strings.ToLower(strings.TrimSpace(email)) != i.email:
		return ErrInvitationForSomeoneElse
	}
	i.acceptedBy, i.acceptedAt = user, now.UTC()
	return nil
}

// InvitationRepository stores invitations in the workspace of ctx (ByTokenHash looks across workspaces when
// ctx has none: the invitee is not a member yet).
type InvitationRepository interface {
	// Save inserts or updates an invitation; a new pending one for the same project and email replaces the old.
	Save(ctx context.Context, i *Invitation) error
	ByTokenHash(ctx context.Context, hash string) (*Invitation, error) // ErrInvitationNotFound
	ByID(ctx context.Context, id InvitationID) (*Invitation, error)    // ErrInvitationNotFound
	// Pending lists the project's invitations not accepted yet (expired ones included), newest first.
	Pending(ctx context.Context, project string) ([]*Invitation, error)
	Delete(ctx context.Context, id InvitationID) error
}
