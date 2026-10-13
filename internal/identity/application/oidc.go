package application

import (
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
