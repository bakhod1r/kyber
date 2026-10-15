// Package domain models SSO connections (ADR-0018): identity providers stored in the database,
// so an installation can sign people in through several of them without a restart.
package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidConnection  = errors.New("invalid SSO connection")
	ErrConnectionNotFound = errors.New("SSO connection not found")
	// ErrNoEncryptionKey: secrets cannot be stored without KYBER_ENCRYPTION_KEY.
	ErrNoEncryptionKey = errors.New("KYBER_ENCRYPTION_KEY is not set, so SSO connections cannot be stored")
)

type (
	ConnectionID string
	Kind         string
)

// KindOIDC is an OpenID Connect provider; SAML connections follow (ADR-0018).
const KindOIDC Kind = "oidc"

// Settings is what an administrator enters for a connection.
type Settings struct {
	Name         string // the sign-in button's label
	Issuer       string
	ClientID     string
	ClientSecret string
	GroupsClaim  string // the ID token claim listing groups (S106); "" = "groups"
	Workspace    string // "" = installation-wide
}

// Connection is one identity provider people can sign in with.
type Connection struct {
	ID ConnectionID
	Kind
	Settings
	Enabled   bool
	CreatedAt time.Time
}

// ProviderOf names a connection's identities, so subjects of two IdPs never collide.
func ProviderOf(id ConnectionID) string { return "sso:" + string(id) }

func NewConnection(id ConnectionID, s Settings, now time.Time) (*Connection, error) {
	s.Name = strings.TrimSpace(s.Name)
	s.Issuer = strings.TrimSpace(s.Issuer)
	s.ClientID = strings.TrimSpace(s.ClientID)
	s.GroupsClaim = strings.TrimSpace(s.GroupsClaim)
	switch {
	case id == "":
		return nil, fmt.Errorf("%w: id is required", ErrInvalidConnection)
	case s.ClientID == "" || s.ClientSecret == "":
		return nil, fmt.Errorf("%w: client id and client secret are required", ErrInvalidConnection)
	}
	if err := checkName(s.Name); err != nil {
		return nil, err
	}
	if err := checkIssuer(s.Issuer); err != nil {
		return nil, err
	}
	return &Connection{ID: id, Kind: KindOIDC, Settings: s, Enabled: true, CreatedAt: now}, nil
}

func checkName(n string) error {
	if n == "" || utf8.RuneCountInString(n) > 60 {
		return fmt.Errorf("%w: name must be 1-60 characters", ErrInvalidConnection)
	}
	return nil
}

// checkIssuer accepts an https URL (http only for a local test IdP) without query or fragment.
func checkIssuer(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: issuer must be a URL", ErrInvalidConnection)
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return fmt.Errorf("%w: issuer must use https", ErrInvalidConnection)
	}
	return nil
}

func (c *Connection) Rename(name string) error {
	name = strings.TrimSpace(name)
	if err := checkName(name); err != nil {
		return err
	}
	c.Name = name
	return nil
}

func (c *Connection) Enable()  { c.Enabled = true }
func (c *Connection) Disable() { c.Enabled = false }

// Connections is the repository port.
type Connections interface {
	All(ctx context.Context) ([]Connection, error)
	// ByID returns ErrConnectionNotFound for an unknown id.
	ByID(ctx context.Context, id ConnectionID) (*Connection, error)
	Save(ctx context.Context, c *Connection) error
	// Delete returns ErrConnectionNotFound for an unknown id.
	Delete(ctx context.Context, id ConnectionID) error
}
