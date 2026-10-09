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
	u, err := s.findOrCreate(ctx, p)
	if errors.Is(err, errLostRace) { // a concurrent first login created the account: use it
		u, err = s.findOrCreate(ctx, p)
	}
	return u, err
}

// syntheticDomain addresses belong to provider accounts without an email; signup refuses it.
const syntheticDomain = "users.kyber.invalid"

var errLostRace = errors.New("account created concurrently")

func (s *Service) findOrCreate(ctx context.Context, p ExternalProfile) (*domain.User, error) {
	id, err := s.external.LinkedUser(ctx, p.Provider, p.Subject)
	if err == nil {
		return s.users.ByID(ctx, id)
	}
	if !errors.Is(err, domain.ErrNotLinked) {
		return nil, err
	}
	// Providers without a (valid) email get an address only this provider account can own.
	synthetic := domain.Email(fmt.Sprintf("%s-%s@%s", p.Provider, p.Subject, syntheticDomain))
	email, emailErr := domain.ParseEmail(p.Email)
	if emailErr != nil {
		email = synthetic
	}
	u, err := s.users.ByEmail(ctx, email)
	switch {
	case err == nil && !p.EmailVerified && email != synthetic:
		return nil, domain.ErrEmailTaken // unverified emails never take over accounts
	case errors.Is(err, domain.ErrUserNotFound):
		id := domain.UserID(s.newID())
		if u, err = domain.NewUser(id, email, p.Name, ""); errors.Is(err, domain.ErrEmptyName) {
			u, _ = domain.NewUser(id, email, fmt.Sprintf("%s user %s", providerTitle[p.Provider], p.Subject), "")
		}
		if err := s.users.Create(ctx, u); errors.Is(err, domain.ErrEmailTaken) {
			return nil, errLostRace
		} else if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	// Linking is idempotent, so a link lost to a failure is repaired on the next login.
	return u, s.external.Link(ctx, domain.ExternalIdentity{Provider: p.Provider, Subject: p.Subject, UserID: u.ID()})
}
