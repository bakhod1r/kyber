// Package application manages stored SSO connections (ADR-0018): site admins add, rename,
// enable, disable and delete identity providers; every change is audited.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

var (
	// ErrForbidden: site admins manage every connection; workspace admins only their workspace's.
	ErrForbidden = errors.New("you cannot manage this SSO connection")
	// ErrSSORequired: the workspace requires its SSO connection (see SSORequiredError).
	ErrSSORequired = errors.New("this workspace requires single sign-on")
)

// SSORequiredError names the connection to sign in with.
type SSORequiredError struct{ ID, Name string }

func (e *SSORequiredError) Error() string { return ErrSSORequired.Error() + " with " + e.Name }
func (e *SSORequiredError) Unwrap() error { return ErrSSORequired }

type (
	// Admins answers who may manage connections and who keeps other ways into a workspace.
	Admins interface {
		IsSiteAdmin(ctx context.Context, user string) bool
		Administers(ctx context.Context, workspace, user string) bool // owner or admin
		Owns(ctx context.Context, workspace, user string) bool
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

// manages: site admins manage every connection; a workspace's admins manage its own (workspace "" =
// installation-wide).
func (s *Service) manages(ctx context.Context, actor, workspace string) error {
	if s.d.Admins.IsSiteAdmin(ctx, actor) || workspace != "" && s.d.Admins.Administers(ctx, workspace, actor) {
		return nil
	}
	return ErrForbidden
}

// List shows site admins every connection, and a workspace's admins that workspace's own.
func (s *Service) List(ctx context.Context, actor, workspace string) ([]domain.Connection, error) {
	if err := s.manages(ctx, actor, workspace); err != nil {
		return nil, err
	}
	all, err := s.d.Connections.All(ctx)
	if err != nil || s.d.Admins.IsSiteAdmin(ctx, actor) {
		return all, err
	}
	own := []domain.Connection{}
	for _, c := range all {
		if c.Workspace == workspace {
			own = append(own, c)
		}
	}
	return own, nil
}

// Create adds a connection for in.Workspace ("" = installation-wide, site admins only).
func (s *Service) Create(ctx context.Context, actor string, in domain.Settings) (*domain.Connection, error) {
	if err := s.manages(ctx, actor, in.Workspace); err != nil {
		return nil, err
	}
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
	Name     *string
	Enabled  *bool
	Enforced *bool
}

func (s *Service) Update(ctx context.Context, actor string, id domain.ConnectionID, ch Change) (*domain.Connection, error) {
	return s.edit(ctx, actor, id, "sso.connection_updated", func(c *domain.Connection) error {
		if ch.Name != nil {
			if err := c.Rename(*ch.Name); err != nil {
				return err
			}
		}
		if ch.Enabled != nil && *ch.Enabled {
			c.Enable()
		} else if ch.Enabled != nil {
			c.Disable()
		}
		if ch.Enforced != nil {
			return c.Enforce(*ch.Enforced)
		}
		return nil
	})
}

// Delete removes a connection. Accounts linked through it keep working with their other sign-in
// methods; their links to it are orphaned and harmless.
func (s *Service) Delete(ctx context.Context, actor string, id domain.ConnectionID) error {
	c, err := s.d.Connections.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.manages(ctx, actor, c.Workspace); err != nil {
		return err
	}
	if err := s.d.Connections.Delete(ctx, id); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, actor, "sso.connection_deleted", *c)
}

// edit loads a connection for a site admin, applies change, saves and audits it.
func (s *Service) edit(ctx context.Context, actor string, id domain.ConnectionID, action string, change func(*domain.Connection) error) (*domain.Connection, error) {
	c, err := s.d.Connections.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.manages(ctx, actor, c.Workspace); err != nil {
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
	if trusted && !s.d.Admins.IsSiteAdmin(ctx, actor) {
		return nil, ErrForbidden // only the installation's operator may vouch for a domain
	}
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

// Check lets the actor into workspace unless the workspace requires a connection the actor did not
// sign in with (method "sso:<id>"); its owners always get in, so nobody is locked out by a broken IdP.
// An API token counts as the sign-in it was created under, so only tokens made through the
// connection get in (ADR-0018 §5).
func (s *Service) Check(ctx context.Context, workspace, actor, method string) error {
	required, err := s.d.Connections.RequiredBy(ctx, workspace)
	if err != nil {
		return err
	}
	for _, c := range required {
		if method == domain.ProviderOf(c.ID) || s.d.Admins.Owns(ctx, workspace, actor) {
			continue
		}
		return &SSORequiredError{ID: string(c.ID), Name: c.Name}
	}
	return nil
}
