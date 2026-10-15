package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var (
	// ErrOAuthDisabled: the server was started without an OAuth store.
	ErrOAuthDisabled = errors.New("OAuth is not enabled")
	// ErrInvalidGrant is the token endpoint's single failure (RFC 6749 invalid_grant): the code is
	// unknown, used, expired, or does not match the client, redirect or PKCE verifier.
	ErrInvalidGrant = errors.New("invalid_grant")
)

// OAuthTokenDays is the lifetime of an access token issued to an MCP client.
const OAuthTokenDays = 90

// WithOAuth enables OAuth 2.1 for MCP clients (ADR-0016).
func WithOAuth(s domain.OAuthStore) Option { return func(svc *Service) { svc.oauth = s } }

// RegisterClient registers a public client (RFC 7591 dynamic client registration).
// Registration is anonymous, so it is limited per client IP with the login limiter's rule
// (LoginAttempts per LoginWindow) under its own key; over it, TooManyAttemptsError.
// A limiter outage fails closed.
func (s *Service) RegisterClient(ctx context.Context, ip, name string, redirects []string) (*domain.OAuthClient, error) {
	if s.oauth == nil {
		return nil, ErrOAuthDisabled
	}
	allowed, retryAfter, err := s.limiter.Allow(ctx, "oauth-register:"+ip)
	if err != nil {
		return nil, fmt.Errorf("registration rate limiter: %w", err)
	}
	if !allowed {
		return nil, &TooManyAttemptsError{RetryAfter: retryAfter}
	}
	c, err := domain.NewOAuthClient(domain.ClientID(s.newID()), name, redirects, s.clock.Now().UTC())
	if err != nil {
		return nil, err
	}
	return c, s.oauth.CreateClient(ctx, c)
}

// OAuthClient looks a client up for the consent screen.
func (s *Service) OAuthClient(ctx context.Context, id string) (*domain.OAuthClient, error) {
	if s.oauth == nil {
		return nil, ErrOAuthDisabled
	}
	return s.oauth.Client(ctx, domain.ClientID(id))
}

// AuthorizeRequest is the user's approval of a client's authorization request.
type AuthorizeRequest struct {
	ClientID, RedirectURI string
	Challenge, Method     string // PKCE: only S256
}

// Authorize issues a single-use authorization code for the signed-in user.
func (s *Service) Authorize(ctx context.Context, user domain.UserID, r AuthorizeRequest) (string, error) {
	c, err := s.OAuthClient(ctx, r.ClientID)
	if err != nil {
		return "", err
	}
	if !c.AllowsRedirect(r.RedirectURI) {
		return "", fmt.Errorf("%w: redirect_uri is not registered for this client", domain.ErrInvalidOAuthRequest)
	}
	if r.Method != "S256" {
		return "", fmt.Errorf("%w: code_challenge_method must be S256", domain.ErrInvalidOAuthRequest)
	}
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	secret := base64.RawURLEncoding.EncodeToString(raw[:])
	code, err := domain.NewAuthCode(hashToken(secret), c.ID, user, r.RedirectURI, r.Challenge, s.clock.Now().UTC())
	if err != nil {
		return "", err
	}
	return secret, s.oauth.SaveCode(ctx, code)
}

// TokenRequest is an authorization_code grant.
type TokenRequest struct {
	Code, ClientID, RedirectURI, Verifier string
}

// ExchangeCode trades a code for an access token: a personal access token named after the client,
// so the user sees and can revoke it under Settings → Access tokens.
func (s *Service) ExchangeCode(ctx context.Context, r TokenRequest) (string, time.Duration, error) {
	if s.oauth == nil {
		return "", 0, ErrOAuthDisabled
	}
	code, err := s.oauth.TakeCode(ctx, hashToken(r.Code))
	if errors.Is(err, domain.ErrAuthCodeNotFound) {
		return "", 0, ErrInvalidGrant
	}
	if err != nil {
		return "", 0, err
	}
	if string(code.Client) != r.ClientID || code.RedirectURI != r.RedirectURI || code.Expired(s.clock.Now()) || !code.Verifier(r.Verifier) {
		return "", 0, ErrInvalidGrant
	}
	if _, err := s.activeUser(ctx, code.User); errors.Is(err, ErrUnauthenticated) { // deactivated or deleted since consent
		return "", 0, ErrInvalidGrant
	} else if err != nil {
		return "", 0, err
	}
	c, err := s.oauth.Client(ctx, code.Client)
	if err != nil {
		return "", 0, err
	}
	secret, _, err := s.CreateToken(ctx, code.User, "MCP: "+c.Name, OAuthTokenDays)
	if err != nil {
		return "", 0, err
	}
	return secret, OAuthTokenDays * 24 * time.Hour, nil
}
