package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var (
	ErrExternalDisabled = errors.New("external sign-in is not configured")
	ErrInvalidProfile   = errors.New("invalid external profile")
)

// WithExternalIdentities enables sign-in with Google and Telegram.
func WithExternalIdentities(ids domain.ExternalIdentities) Option {
	return func(s *Service) { s.external = ids }
}

var providerTitle = map[domain.Provider]string{domain.ProviderGoogle: "Google", domain.ProviderTelegram: "Telegram"}

// ExternalProfile is what an identity provider vouches for after verification by its adapter.
type ExternalProfile struct {
	Provider      domain.Provider
	Subject       string // stable provider user id
	Email         string
	EmailVerified bool
	Name          string
}

// LoginExternal signs a provider-verified user in, creating or linking the account:
// a linked identity wins; otherwise a *verified* email links to the existing account
// (unverified emails never take over accounts); otherwise a password-less account is created.
func (s *Service) LoginExternal(ctx context.Context, p ExternalProfile) (string, *domain.User, error) {
	if s.external == nil {
		return "", nil, ErrExternalDisabled
	}
	if !p.Provider.Valid() || p.Subject == "" {
		return "", nil, ErrInvalidProfile
	}
	u, err := s.externalUser(ctx, p)
	if err != nil {
		return "", nil, err
	}
	token, err := s.newSession(ctx, u.ID())
	return token, u, err
}

func (s *Service) externalUser(ctx context.Context, p ExternalProfile) (*domain.User, error) {
	id, err := s.external.LinkedUser(ctx, p.Provider, p.Subject)
	if err == nil {
		return s.users.ByID(ctx, id)
	}
	if !errors.Is(err, domain.ErrNotLinked) {
		return nil, err
	}
	email, emailErr := domain.ParseEmail(p.Email)
	if emailErr != nil {
		email = domain.Email(fmt.Sprintf("%s-%s@users.kyber.invalid", p.Provider, p.Subject))
	}
	u, err := s.users.ByEmail(ctx, email)
	switch {
	case err == nil && !p.EmailVerified:
		return nil, domain.ErrEmailTaken
	case errors.Is(err, domain.ErrUserNotFound):
		id := domain.UserID(s.newID())
		if u, err = domain.NewUser(id, email, p.Name, ""); errors.Is(err, domain.ErrEmptyName) {
			u, _ = domain.NewUser(id, email, fmt.Sprintf("%s user %s", providerTitle[p.Provider], p.Subject), "")
		}
		if err := s.users.Create(ctx, u); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	return u, s.external.Link(ctx, domain.ExternalIdentity{Provider: p.Provider, Subject: p.Subject, UserID: u.ID()})
}
