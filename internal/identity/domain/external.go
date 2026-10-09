package domain

import (
	"context"
	"errors"
)

var ErrNotLinked = errors.New("external identity not linked")

// Provider is an external identity provider users can sign in with.
type Provider string

const (
	ProviderGoogle   Provider = "google"
	ProviderTelegram Provider = "telegram"
)

func (p Provider) Valid() bool { return p == ProviderGoogle || p == ProviderTelegram }

// ExternalIdentity links a provider account (its stable subject id) to a Kyber user.
type ExternalIdentity struct {
	Provider Provider
	Subject  string
	UserID   UserID
}

type ExternalIdentities interface {
	// LinkedUser returns the user linked to (provider, subject) or ErrNotLinked.
	LinkedUser(ctx context.Context, p Provider, subject string) (UserID, error)
	Link(ctx context.Context, id ExternalIdentity) error
}
