// Package acl is the Chat context's anti-corruption layer to Workspace and Identity.
package acl

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/chat/application"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
)

type Membership interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

type Users interface {
	ByID(ctx context.Context, id identitydomain.UserID) (*identitydomain.User, error)
}

// Members: someone in a conversation must be a real user and a member of the workspace (the
// default workspace is open to every user, so existence is checked separately).
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

// IssueReader is Issue Tracking's authorized read (browse permission and security level).
type IssueReader interface {
	Get(ctx context.Context, actor, rawKey string) (*issuedomain.Issue, error)
}

// Issues previews issues for chat readers; anything the reader may not browse (or that does not
// exist) is simply not previewed.
type Issues struct{ src IssueReader }

func NewIssues(src IssueReader) Issues { return Issues{src: src} }

func (i Issues) Preview(ctx context.Context, user, key string) (application.IssuePreview, bool, error) {
	is, err := i.src.Get(ctx, user, key)
	if err != nil {
		return application.IssuePreview{}, false, nil
	}
	return application.IssuePreview{Key: is.Key().String(), Title: is.Title(), Status: string(is.Status())}, true, nil
}
