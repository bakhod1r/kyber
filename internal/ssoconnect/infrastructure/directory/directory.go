// Package directory serves the enabled SSO connections to sign-in (the Identity context's
// SSODirectory port), building one OpenID Connect client per connection and rebuilding it
// when the connection changes.
package directory

import (
	"context"
	"errors"
	"sync"

	identity "github.com/bakhod1r/kyber/internal/identity/application"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/oidcprovider"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// Build makes the sign-in client of a connection.
type Build func(c domain.Connection) identity.RedirectProvider

// OIDC builds clients with the OpenID Connect adapter; callback gives each connection's redirect URI.
func OIDC(callback func(id string) string) Build {
	return func(c domain.Connection) identity.RedirectProvider {
		return oidcprovider.New(oidcprovider.Config{Provider: identitydomain.Provider(domain.ProviderOf(c.ID)), Issuer: c.Issuer,
			ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: callback(string(c.ID)), RequireVerifiedEmail: true,
			GroupsClaim: c.GroupsClaim})
	}
}

type entry struct {
	c domain.Connection
	p identity.RedirectProvider
}

type Directory struct {
	repo  domain.Connections
	build Build
	mu    sync.Mutex
	cache map[domain.ConnectionID]entry
}

func New(repo domain.Connections, build Build) *Directory {
	return &Directory{repo: repo, build: build, cache: map[domain.ConnectionID]entry{}}
}

// provider reuses a client while its connection is unchanged (discovery is done once per client).
func (d *Directory) provider(c domain.Connection) identity.RedirectProvider {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.cache[c.ID]; ok && e.c == c {
		return e.p
	}
	p := d.build(c)
	d.cache[c.ID] = entry{c: c, p: p}
	return p
}

func (d *Directory) SSOConnections(ctx context.Context) ([]identity.SSOConnection, error) {
	all, err := d.repo.All(ctx)
	if err != nil {
		return nil, err
	}
	out := []identity.SSOConnection{}
	for _, c := range all {
		if c.Enabled {
			out = append(out, identity.SSOConnection{ID: string(c.ID), Name: c.Name, Provider: d.provider(c)})
		}
	}
	return out, nil
}

func (d *Directory) SSOConnection(ctx context.Context, id string) (identity.SSOConnection, error) {
	c, err := d.repo.ByID(ctx, domain.ConnectionID(id))
	if errors.Is(err, domain.ErrConnectionNotFound) || (err == nil && !c.Enabled) {
		return identity.SSOConnection{}, identity.ErrSSOConnectionNotFound
	}
	if err != nil {
		return identity.SSOConnection{}, err
	}
	return identity.SSOConnection{ID: id, Name: c.Name, Provider: d.provider(*c)}, nil
}

var _ identity.SSODirectory = (*Directory)(nil)
