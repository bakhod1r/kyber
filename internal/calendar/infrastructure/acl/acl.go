// Package acl is the Calendar context's anti-corruption layer to Workspace and Identity.
package acl

import (
	"context"
	"errors"

	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
)

type Membership interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

type Users interface {
	ByID(ctx context.Context, id identitydomain.UserID) (*identitydomain.User, error)
}

// Members: an attendee must be a real user and a member of the workspace (the default
// workspace is open to every user, so existence is checked separately).
type Members struct {
	ws    Membership
	users Users
}

func NewMembers(ws Membership, users Users) Members { return Members{ws: ws, users: users} }

func (m Members) IsMember(ctx context.Context, workspace, user string) (bool, error) {
	if _, err := m.users.ByID(ctx, identitydomain.UserID(user)); errors.Is(err, identitydomain.ErrUserNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return m.ws.IsMember(ctx, workspace, user)
}
