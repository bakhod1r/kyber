// Package google signs users in with Google: the generic OpenID Connect adapter (oidcprovider)
// preset to Google's issuer and account chooser.
package google

import (
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/oidcprovider"
)

// Issuer is Google's OpenID Connect issuer.
const Issuer = "https://accounts.google.com"

// ErrInvalidLogin is the application layer's error for a failed external sign-in.
var ErrInvalidLogin = oidcprovider.ErrInvalidLogin

type (
	Client = oidcprovider.Client
	Flow   = oidcprovider.Flow
)

type Config struct {
	Issuer       string // default Issuer; tests point it at a fake provider
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g. https://kyber.example.com/api/v1/auth/google/callback
}

func New(cfg Config) *Client {
	if cfg.Issuer == "" {
		cfg.Issuer = Issuer
	}
	return oidcprovider.New(oidcprovider.Config{Provider: domain.ProviderGoogle, Issuer: cfg.Issuer, ClientID: cfg.ClientID,
		ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL, AuthParams: map[string]string{"prompt": "select_account"}})
}
