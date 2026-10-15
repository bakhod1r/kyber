package domain

import (
	"context"
	"errors"
	"strings"
)

var ErrNotLinked = errors.New("external identity not linked")

// Provider is an external identity provider users can sign in with.
type Provider string

const (
	ProviderGoogle   Provider = "google"
	ProviderTelegram Provider = "telegram"
	ProviderOIDC     Provider = "oidc"   // the installation's generic OpenID Connect provider (SSO)
	ProviderGitHub   Provider = "github" // subject = GitHub's numeric user id
	ProviderSAML     Provider = "saml"   // the installation's SAML identity provider; subject = NameID

	ssoPrefix = "sso:" // a stored SSO connection (ADR-0018): "sso:<connection id>"
)

func (p Provider) Valid() bool {
	switch p {
	case ProviderGoogle, ProviderTelegram, ProviderOIDC, ProviderGitHub, ProviderSAML:
		return true
	}
	return strings.HasPrefix(string(p), ssoPrefix) && len(p) > len(ssoPrefix)
}

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
	// SubjectOf returns the user's account id at the provider (e.g. their Telegram chat) or ErrNotLinked.
	SubjectOf(ctx context.Context, user UserID, p Provider) (string, error)
}
