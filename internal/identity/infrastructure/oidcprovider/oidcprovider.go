// Package oidcprovider signs users in with any OpenID Connect provider (authorization code flow
// with PKCE, state and nonce). The provider is found by discovery
// (<issuer>/.well-known/openid-configuration) and the ID token's signature, issuer, audience,
// expiry and nonce are verified against the provider's published keys (JWKS).
package oidcprovider

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// ErrInvalidLogin is the application layer's error for a failed external sign-in.
var ErrInvalidLogin = application.ErrInvalidLogin

// Flow is the application layer's OpenID Connect flow.
type Flow = application.OIDCFlow

type Config struct {
	Provider     domain.Provider // whose identities these are
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g. https://kyber.example.com/api/v1/auth/oidc/callback
	// RequireVerifiedEmail refuses ID tokens whose email_verified claim is not true.
	RequireVerifiedEmail bool
	// AuthParams are extra authorization request parameters (e.g. prompt=select_account).
	AuthParams map[string]string
	// GroupsClaim names the ID token claim listing the user's groups (S106); "" = "groups".
	GroupsClaim string
}

// Client discovers the provider lazily, so the server starts even when it is unreachable.
type Client struct {
	cfg      Config
	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func New(cfg Config) *Client { return &Client{cfg: cfg} }

func (c *Client) Enabled() bool {
	return c.cfg.Issuer != "" && c.cfg.ClientID != "" && c.cfg.ClientSecret != ""
}

func (c *Client) discover(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.oauth != nil {
		return c.oauth, c.verifier, nil
	}
	p, err := oidc.NewProvider(ctx, c.cfg.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("%s discovery: %w", c.cfg.Provider, err)
	}
	c.oauth = &oauth2.Config{ClientID: c.cfg.ClientID, ClientSecret: c.cfg.ClientSecret, RedirectURL: c.cfg.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}
	c.verifier = p.Verifier(&oidc.Config{ClientID: c.cfg.ClientID})
	return c.oauth, c.verifier, nil
}

// AuthURL is where the browser is sent to consent.
func (c *Client) AuthURL(ctx context.Context, f Flow) (string, error) {
	o, _, err := c.discover(ctx)
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{oidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier)}
	for k, v := range c.cfg.AuthParams {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	return o.AuthCodeURL(f.State, opts...), nil
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
	if c.cfg.RequireVerifiedEmail && !claims.EmailVerified {
		return application.ExternalProfile{}, fmt.Errorf("%w: email not verified", ErrInvalidLogin)
	}
	var all map[string]any
	_ = id.Claims(&all)
	return application.ExternalProfile{Provider: c.cfg.Provider, Subject: id.Subject, Email: claims.Email,
		EmailVerified: claims.EmailVerified, Name: claims.Name, Groups: groups(all[c.groupsClaim()])}, nil
}

func (c *Client) groupsClaim() string {
	if c.cfg.GroupsClaim != "" {
		return c.cfg.GroupsClaim
	}
	return "groups"
}

// groups reads a groups claim: a list of strings or one string; anything else is no claim (nil).
func groups(v any) []string {
	switch g := v.(type) {
	case string:
		return []string{g}
	case []any:
		out := make([]string, 0, len(g))
		for _, x := range g {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
