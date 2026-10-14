package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// ErrTokenLifetime: tokens live 1-365 days, or forever when no lifetime is given.
var ErrTokenLifetime = errors.New("token lifetime must be 1-365 days")

// MaxTokenDays bounds a personal access token's lifetime.
const MaxTokenDays = 365

// WithPersonalTokens enables personal access tokens (ADR-0012).
func WithPersonalTokens(t domain.PersonalTokens) Option { return func(s *Service) { s.tokens = t } }

// CreateToken issues a personal access token for the user. The secret is returned once; only
// its hash is stored. days = 0 means it never expires.
func (s *Service) CreateToken(ctx context.Context, user domain.UserID, name string, days int) (string, *domain.PersonalToken, error) {
	if days < 0 || days > MaxTokenDays {
		return "", nil, fmt.Errorf("%w: %w", domain.ErrInvalidToken, ErrTokenLifetime)
	}
	now := s.clock.Now().UTC()
	var expires *time.Time
	if days > 0 {
		e := now.AddDate(0, 0, days)
		expires = &e
	}
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	secret := domain.TokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
	t, err := domain.NewPersonalToken(domain.TokenID(s.newID()), user, name, hashToken(secret), now, expires)
	if err != nil {
		return "", nil, err
	}
	if err := s.tokens.CreateToken(ctx, t); err != nil {
		return "", nil, err
	}
	return secret, t, nil
}

// Tokens lists the user's personal access tokens, newest first.
func (s *Service) Tokens(ctx context.Context, user domain.UserID) ([]*domain.PersonalToken, error) {
	return s.tokens.TokensOf(ctx, user)
}

// RevokeToken deletes one of the user's tokens.
func (s *Service) RevokeToken(ctx context.Context, user domain.UserID, id string) error {
	return s.tokens.DeleteToken(ctx, user, domain.TokenID(id))
}

// IsPersonalToken tells personal access tokens from session tokens.
func IsPersonalToken(token string) bool { return strings.HasPrefix(token, domain.TokenPrefix) }

// authenticateToken resolves a personal access token to its owner, recording its use.
func (s *Service) authenticateToken(ctx context.Context, secret string) (*domain.User, error) {
	if s.tokens == nil {
		return nil, ErrUnauthenticated
	}
	t, err := s.tokens.TokenByHash(ctx, hashToken(secret))
	if errors.Is(err, domain.ErrTokenNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if t.Expired(now) {
		return nil, ErrUnauthenticated
	}
	if t.TouchDue(now) {
		if err := s.tokens.TouchToken(ctx, t.ID, now.UTC()); err != nil {
			return nil, err
		}
	}
	u, err := s.users.ByID(ctx, t.UserID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil, ErrUnauthenticated
	}
	return u, err
}

// botDomain gives bots placeholder addresses that are never mailed (RFC 2606 .invalid).
const botDomain = "bots.kyber.invalid"

// CreateBot makes a bot user (S84): no password, so it never signs in, and a token that does not expire,
// returned once.
func (s *Service) CreateBot(ctx context.Context, name string) (*domain.User, string, error) {
	id := domain.UserID(s.newID())
	u, err := domain.NewUser(id, domain.Email("bot-"+string(id)+"@"+botDomain), name, "")
	if err != nil {
		return nil, "", err
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, "", err
	}
	secret, _, err := s.CreateToken(ctx, u.ID(), "bot", 0)
	if err != nil {
		return nil, "", err
	}
	return u, secret, nil
}
