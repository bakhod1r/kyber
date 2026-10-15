package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// ErrInvalidLogin: an external (OpenID Connect) sign-in failed verification.
var ErrInvalidLogin = errors.New("invalid external login")

// OIDCFlow holds the per-attempt secrets of an OpenID Connect sign-in (state, nonce and PKCE
// verifier) that the browser keeps in a short-lived cookie.
type OIDCFlow struct{ State, Nonce, Verifier string }

// NewOIDCFlow creates fresh random state, nonce and PKCE verifier (32 random bytes each).
func NewOIDCFlow() OIDCFlow {
	return OIDCFlow{State: random(), Nonce: random(), Verifier: random()}
}

func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ErrSSOConnectionNotFound: no enabled SSO connection has this id.
var ErrSSOConnectionNotFound = errors.New("SSO connection not found")

type (
	// RedirectProvider is an authorization-code sign-in provider (OpenID Connect).
	RedirectProvider interface {
		AuthURL(ctx context.Context, f OIDCFlow) (string, error)
		Exchange(ctx context.Context, code string, f OIDCFlow) (ExternalProfile, error)
	}
	// SSOConnection is one stored identity provider (ADR-0018) as sign-in sees it.
	SSOConnection struct {
		ID, Name string
		Provider RedirectProvider
	}
	// SSODirectory lists the enabled stored connections (port to the SSO connections context).
	SSODirectory interface {
		SSOConnections(ctx context.Context) ([]SSOConnection, error)
		// SSOConnection returns ErrSSOConnectionNotFound for an unknown or disabled id.
		SSOConnection(ctx context.Context, id string) (SSOConnection, error)
	}
)
