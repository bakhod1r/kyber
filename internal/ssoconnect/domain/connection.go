// Package domain models SSO connections (ADR-0018): identity providers stored in the database,
// so an installation can sign people in through several of them without a restart.
package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidConnection  = errors.New("invalid SSO connection")
	ErrConnectionNotFound = errors.New("SSO connection not found")
	// ErrNoEncryptionKey: secrets cannot be stored without KYBER_ENCRYPTION_KEY.
	ErrNoEncryptionKey = errors.New("KYBER_ENCRYPTION_KEY is not set, so SSO connections cannot be stored")
	ErrDomainNotFound  = errors.New("this connection has no such domain")
	// ErrDomainNotVerified: the domain's DNS has no TXT record "kyber-verify=<token>" yet.
	ErrDomainNotVerified = errors.New("the domain's TXT record kyber-verify=<token> was not found")
	// ErrDomainTaken: another connection already routes this domain.
	ErrDomainTaken = errors.New("another SSO connection already uses this domain")
)

// Domain is an email domain a connection serves (ADR-0018 §4). It routes sign-in only once
// verified: by a DNS TXT record "kyber-verify=<Token>", or trusted by a site admin.
type Domain struct {
	Name     string
	Token    string
	Verified bool
}

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
	Domains   []Domain // sorted by name
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

var hostname = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// DomainOf returns the lower-cased domain of an email address, or "".
func DomainOf(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}

func (c *Connection) find(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	for i, d := range c.Domains {
		if d.Name == name {
			return i
		}
	}
	return -1
}

// AddDomain adds an unverified domain (trusted: verified at once, for site admins who own the install).
func (c *Connection) AddDomain(name, token string, trusted bool) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !hostname.MatchString(name) || len(name) > 253 {
		return fmt.Errorf("%w: %q is not a domain name", ErrInvalidConnection, name)
	}
	if c.find(name) >= 0 {
		return fmt.Errorf("%w: %s is already listed", ErrInvalidConnection, name)
	}
	c.Domains = append(c.Domains, Domain{Name: name, Token: token, Verified: trusted})
	slices.SortFunc(c.Domains, func(a, b Domain) int { return strings.Compare(a.Name, b.Name) })
	return nil
}

// VerifyDomain marks the domain verified when one of its TXT records is "kyber-verify=<token>".
func (c *Connection) VerifyDomain(name string, txt []string) error {
	i := c.find(name)
	if i < 0 {
		return ErrDomainNotFound
	}
	for _, r := range txt {
		if strings.TrimSpace(r) == "kyber-verify="+c.Domains[i].Token {
			c.Domains[i].Verified = true
			return nil
		}
	}
	return ErrDomainNotVerified
}

func (c *Connection) RemoveDomain(name string) error {
	i := c.find(name)
	if i < 0 {
		return ErrDomainNotFound
	}
	c.Domains = append(c.Domains[:i], c.Domains[i+1:]...)
	return nil
}

// Serves reports whether the connection routes sign-in for the (verified) domain.
func (c *Connection) Serves(name string) bool {
	i := c.find(name)
	return i >= 0 && c.Domains[i].Verified
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
	// ByDomain returns the connection listing the domain (verified or not) or ErrConnectionNotFound.
	ByDomain(ctx context.Context, domain string) (*Connection, error)
}
