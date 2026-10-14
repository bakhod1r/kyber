package domain

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// OAuth 2.1 for MCP clients (ADR-0016): clients register themselves, a signed-in user approves
// them, and the authorization code (PKCE S256 only) is exchanged for a personal access token.
var (
	ErrInvalidOAuthClient  = errors.New("invalid OAuth client")
	ErrOAuthClientNotFound = errors.New("OAuth client not found")
	ErrInvalidOAuthRequest = errors.New("invalid OAuth request")
	ErrAuthCodeNotFound    = errors.New("authorization code not found")
)

type ClientID string

// OAuthClient is a public client (no secret) registered dynamically (RFC 7591).
type OAuthClient struct {
	ID           ClientID
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

const maxRedirects = 10

// NewOAuthClient accepts https redirects, loopback http ones and private-use schemes with a dot
// (com.example.app:/cb), as RFC 8252 allows for native apps.
func NewOAuthClient(id ClientID, name string, redirects []string, at time.Time) (*OAuthClient, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 || strings.ContainsAny(name, "\r\n") {
		return nil, fmt.Errorf("%w: client_name must be one line of 1-100 characters", ErrInvalidOAuthClient)
	}
	if len(redirects) == 0 || len(redirects) > maxRedirects {
		return nil, fmt.Errorf("%w: 1-%d redirect_uris are required", ErrInvalidOAuthClient, maxRedirects)
	}
	for _, r := range redirects {
		if !validRedirect(r) {
			return nil, fmt.Errorf("%w: redirect_uri %q must be https, loopback http or a private-use scheme", ErrInvalidOAuthClient, r)
		}
	}
	return &OAuthClient{ID: id, Name: name, RedirectURIs: slices.Clone(redirects), CreatedAt: at}, nil
}

var privateScheme = regexp.MustCompile(`^[a-z][a-z0-9+-]*(\.[a-z0-9+-]+)+$`)

func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Scheme == "" {
		return false
	}
	switch {
	case u.Scheme == "https":
		return u.Host != ""
	case u.Scheme == "http":
		return loopback(u.Hostname())
	}
	return privateScheme.MatchString(u.Scheme)
}

func loopback(host string) bool { return host == "localhost" || host == "127.0.0.1" || host == "::1" }

// AllowsRedirect matches a registered redirect exactly, except that a loopback redirect may use
// any port (native apps listen on an ephemeral one, RFC 8252 §7.3).
func (c *OAuthClient) AllowsRedirect(raw string) bool {
	if slices.Contains(c.RedirectURIs, raw) {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || !loopback(u.Hostname()) {
		return false
	}
	for _, r := range c.RedirectURIs {
		reg, _ := url.Parse(r)
		if reg.Scheme == "http" && reg.Hostname() == u.Hostname() && reg.Path == u.Path && reg.RawQuery == u.RawQuery {
			return true
		}
	}
	return false
}

// AuthCodeTTL is how long an authorization code can be exchanged.
const AuthCodeTTL = 10 * time.Minute

// AuthCode is a single-use authorization code; only its hash is stored.
type AuthCode struct {
	Hash        []byte
	Client      ClientID
	User        UserID
	RedirectURI string
	Challenge   string // base64url(SHA-256(verifier))
	ExpiresAt   time.Time
}

var challengeShape = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var verifierShape = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

func NewAuthCode(hash []byte, client ClientID, user UserID, redirect, challenge string, at time.Time) (*AuthCode, error) {
	if !challengeShape.MatchString(challenge) {
		return nil, fmt.Errorf("%w: code_challenge must be an S256 challenge", ErrInvalidOAuthRequest)
	}
	return &AuthCode{Hash: hash, Client: client, User: user, RedirectURI: redirect, Challenge: challenge, ExpiresAt: at.Add(AuthCodeTTL)}, nil
}

// Verifier checks the PKCE code_verifier against the stored challenge (constant time).
func (c *AuthCode) Verifier(verifier string) bool {
	if !verifierShape.MatchString(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(c.Challenge)) == 1
}

func (c *AuthCode) Expired(now time.Time) bool { return !now.Before(c.ExpiresAt) }

// OAuthStore keeps registered clients and pending authorization codes.
type OAuthStore interface {
	CreateClient(ctx context.Context, c *OAuthClient) error
	Client(ctx context.Context, id ClientID) (*OAuthClient, error) // ErrOAuthClientNotFound
	SaveCode(ctx context.Context, c *AuthCode) error
	// TakeCode returns and deletes a code, so it can be used once (ErrAuthCodeNotFound).
	TakeCode(ctx context.Context, hash []byte) (*AuthCode, error)
}
