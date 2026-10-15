// Package application manages stored SSO connections (ADR-0018): site admins add, rename,
// enable, disable and delete identity providers; every change is audited.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// ErrForbidden: only site admins manage installation-wide SSO connections.
var ErrForbidden = errors.New("only site administrators manage SSO connections")

type (
	Admins interface {
		IsSiteAdmin(ctx context.Context, user string) bool
	}
	// Audit records each change (actor, action, connection).
	Audit interface {
		Record(ctx context.Context, actor, action string, c domain.Connection) error
	}
	// DNS reads a domain's TXT records (for domain verification).
	DNS interface {
		TXT(ctx context.Context, name string) ([]string, error)
	}
)

type Deps struct {
	Connections domain.Connections
	Admins      Admins
	Audit       Audit
	DNS         DNS
	NewID       func() string
	NewToken    func() string // the random part of "kyber-verify=<token>"
	Now         func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

func (s *Service) admin(ctx context.Context, actor string) error {
	if !s.d.Admins.IsSiteAdmin(ctx, actor) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) List(ctx context.Context, actor string) ([]domain.Connection, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	return s.d.Connections.All(ctx)
}

func (s *Service) Create(ctx context.Context, actor string, in domain.Settings) (*domain.Connection, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	in.Workspace = "" // installation-wide; per-workspace connections come with ADR-0018 phase (c)
	c, err := domain.NewConnection(domain.ConnectionID(s.d.NewID()), in, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := s.d.Connections.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, s.d.Audit.Record(ctx, actor, "sso.connection_created", *c)
}

// Change is a partial update: nil fields stay as they are.
type Change struct {
	Name    *string
	Enabled *bool
}

func (s *Service) Update(ctx context.Context, actor string, id domain.ConnectionID, ch Change) (*domain.Connection, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	c, err := s.d.Connections.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch.Name != nil {
		if err := c.Rename(*ch.Name); err != nil {
			return nil, err
		}
	}
	if ch.Enabled != nil && *ch.Enabled {
		c.Enable()
	} else if ch.Enabled != nil {
		c.Disable()
	}
	if err := s.d.Connections.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, s.d.Audit.Record(ctx, actor, "sso.connection_updated", *c)
}

// Delete removes a connection. Accounts linked through it keep working with their other sign-in
// methods; their links to it are orphaned and harmless.
func (s *Service) Delete(ctx context.Context, actor string, id domain.ConnectionID) error {
	if err := s.admin(ctx, actor); err != nil {
		return err
	}
	c, err := s.d.Connections.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.d.Connections.Delete(ctx, id); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, actor, "sso.connection_deleted", *c)
}

// edit loads a connection for a site admin, applies change, saves and audits it.
func (s *Service) edit(ctx context.Context, actor string, id domain.ConnectionID, action string, change func(*domain.Connection) error) (*domain.Connection, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	c, err := s.d.Connections.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := change(c); err != nil {
		return nil, err
	}
	if err := s.d.Connections.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, s.d.Audit.Record(ctx, actor, action, *c)
}

// AddDomain lists an email domain for the connection. It routes sign-in once verified; trusted
// skips DNS verification (the site admin vouches for the domain, ADR-0018 §4).
func (s *Service) AddDomain(ctx context.Context, actor string, id domain.ConnectionID, name string, trusted bool) (*domain.Connection, error) {
	return s.edit(ctx, actor, id, "sso.domain_added", func(c *domain.Connection) error {
		return c.AddDomain(name, s.d.NewToken(), trusted)
	})
}

// VerifyDomain looks for the TXT record "kyber-verify=<token>" on the domain. A failed lookup
// counts as "not found yet": DNS changes take time to propagate.
func (s *Service) VerifyDomain(ctx context.Context, actor string, id domain.ConnectionID, name string) (*domain.Connection, error) {
	return s.edit(ctx, actor, id, "sso.domain_verified", func(c *domain.Connection) error {
		txt, err := s.d.DNS.TXT(ctx, name)
		if err != nil {
			txt = nil
		}
		return c.VerifyDomain(name, txt)
	})
}

func (s *Service) RemoveDomain(ctx context.Context, actor string, id domain.ConnectionID, name string) (*domain.Connection, error) {
	return s.edit(ctx, actor, id, "sso.domain_removed", func(c *domain.Connection) error { return c.RemoveDomain(name) })
}

// Discover finds the enabled connection serving the email's verified domain (the login page asks
// for the email first). ErrConnectionNotFound means: sign in another way.
func (s *Service) Discover(ctx context.Context, email string) (*domain.Connection, error) {
	name := domain.DomainOf(email)
	if name == "" {
		return nil, domain.ErrConnectionNotFound
	}
	c, err := s.d.Connections.ByDomain(ctx, name)
	if err != nil {
		return nil, err
	}
	if !c.Enabled || !c.Serves(name) {
		return nil, domain.ErrConnectionNotFound
	}
	return c, nil
}
