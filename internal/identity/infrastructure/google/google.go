// Package google signs users in with Google (OpenID Connect authorization code flow with
// PKCE, state and nonce). The ID token's signature, issuer, audience and expiry are verified
// against Google's published keys.
package google

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// Issuer is Google's OpenID Connect issuer.
const Issuer = "https://accounts.google.com"

// ErrInvalidLogin is the application layer's error for a failed external sign-in.
var ErrInvalidLogin = application.ErrInvalidLogin

type Config struct {
	Issuer       string // default Issuer; tests point it at a fake provider
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g. https://kyber.example.com/api/v1/auth/google/callback
}

// Client discovers the provider lazily, so the server starts even when Google is unreachable.
type Client struct {
	cfg      Config
	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func New(cfg Config) *Client {
	if cfg.Issuer == "" {
		cfg.Issuer = Issuer
	}
	return &Client{cfg: cfg}
}

func (c *Client) Enabled() bool { return c.cfg.ClientID != "" && c.cfg.ClientSecret != "" }

func (c *Client) discover(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.oauth != nil {
		return c.oauth, c.verifier, nil
	}
	p, err := oidc.NewProvider(ctx, c.cfg.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("google discovery: %w", err)
	}
	c.oauth = &oauth2.Config{ClientID: c.cfg.ClientID, ClientSecret: c.cfg.ClientSecret, RedirectURL: c.cfg.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}
	c.verifier = p.Verifier(&oidc.Config{ClientID: c.cfg.ClientID})
	return c.oauth, c.verifier, nil
}

// Flow is the application layer's OpenID Connect flow.
type Flow = application.OIDCFlow

// AuthURL is where the browser is sent to consent.
func (c *Client) AuthURL(ctx context.Context, f Flow) (string, error) {
	o, _, err := c.discover(ctx)
	if err != nil {
		return "", err
	}
	return o.AuthCodeURL(f.State, oidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier),
		oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

// Exchange redeems the code and returns the verified profile.
func (c *Client) Exchange(ctx context.Context, code string, f Flow) (application.ExternalProfile, error) {
	o, v, err := c.discover(ctx)
	if err != nil {
		return application.ExternalProfile{}, err
	}
	tok, err := o.Exchange(ctx, code, oauth2.VerifierOption(f.Verifier))
	if err != nil {
		return application.ExternalProfile{}, fmt.Errorf("%w: code exchange: %w", ErrInvalidLogin, err)
	}
	raw, _ := tok.Extra("id_token").(string)
	id, err := v.Verify(ctx, raw)
	if err != nil {
		return application.ExternalProfile{}, fmt.Errorf("%w: %w", ErrInvalidLogin, err)
	}
	if id.Nonce != f.Nonce {
		return application.ExternalProfile{}, fmt.Errorf("%w: nonce mismatch", ErrInvalidLogin)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	_ = id.Claims(&claims) // the payload was already parsed by Verify
	return application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: id.Subject, Email: claims.Email,
		EmailVerified: claims.EmailVerified, Name: claims.Name}, nil
}
