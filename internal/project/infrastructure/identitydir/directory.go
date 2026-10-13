// Package identitydir is the anti-corruption layer from the Project context to Identity.
package identitydir

import (
	"context"
	"errors"

	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/project/application"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Directory struct{ users identitydomain.Users }

func New(users identitydomain.Users) *Directory { return &Directory{users: users} }

func (d *Directory) ByEmail(ctx context.Context, raw string) (application.UserInfo, error) {
	email, err := identitydomain.ParseEmail(raw)
	if err != nil {
		return application.UserInfo{}, application.ErrUnknownUser
	}
	return translate(d.users.ByEmail(ctx, email))
}

func (d *Directory) ByID(ctx context.Context, id domain.UserID) (application.UserInfo, error) {
	return translate(d.users.ByID(ctx, identitydomain.UserID(id)))
}

func translate(u *identitydomain.User, err error) (application.UserInfo, error) {
	if errors.Is(err, identitydomain.ErrUserNotFound) {
		return application.UserInfo{}, application.ErrUnknownUser
	}
	if err != nil {
		return application.UserInfo{}, err
	}
	return application.UserInfo{ID: domain.UserID(u.ID()), Email: u.Email().String(), Name: u.Name()}, nil
}
