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
)

type Deps struct {
	Connections domain.Connections
	Admins      Admins
	Audit       Audit
	NewID       func() string
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
