// Package application runs SSO group sync (S106): site admins manage group mappings; every SSO
// login applies them to the user.
package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

// ErrForbidden: only site admins manage group mappings.
var ErrForbidden = errors.New("only site administrators manage SSO group mappings")

type (
	// Roles grants and revokes workspace and project roles (ACL to the Workspace and Project contexts).
	Roles interface {
		Grant(ctx context.Context, t domain.Target, user, role string) error
		Revoke(ctx context.Context, t domain.Target, user string) error
	}
	// Audit records each change in the target's audit log.
	Audit interface {
		Record(ctx context.Context, user, action string, t domain.Target, role string) error
	}
	Admins interface {
		IsSiteAdmin(ctx context.Context, user string) bool
	}
)

type Deps struct {
	Mappings domain.Mappings
	Grants   domain.Grants
	Roles    Roles
	Audit    Audit
	Admins   Admins
	NewID    func() string
	Now      func() time.Time
	Log      *slog.Logger
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

func (s *Service) admin(ctx context.Context, actor string) error {
	if !s.d.Admins.IsSiteAdmin(ctx, actor) {
		return ErrForbidden
	}
	return nil
}

func (s *Service) List(ctx context.Context, actor string) ([]domain.Mapping, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	return s.d.Mappings.All(ctx)
}

func (s *Service) Create(ctx context.Context, actor, group string, t domain.Target, role string) (*domain.Mapping, error) {
	if err := s.admin(ctx, actor); err != nil {
		return nil, err
	}
	m, err := domain.NewMapping(domain.MappingID(s.d.NewID()), group, t, role, actor, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	return m, s.d.Mappings.Create(ctx, m)
}

// Delete removes a mapping; roles it granted are revoked at each user's next SSO login.
func (s *Service) Delete(ctx context.Context, actor string, id domain.MappingID) error {
	if err := s.admin(ctx, actor); err != nil {
		return err
	}
	return s.d.Mappings.Delete(ctx, id)
}

// Sync applies the mappings to user's groups (called at every SSO login). It keeps going past a
// failing target, so one deleted project does not block the rest; failures are retried at the
// next login because their grants were not recorded.
func (s *Service) Sync(ctx context.Context, user string, groups []string) error {
	mappings, err := s.d.Mappings.All(ctx)
	if err != nil || len(mappings) == 0 {
		return err
	}
	held, err := s.d.Grants.Of(ctx, user)
	if err != nil {
		return err
	}
	set, revoke := domain.Plan(user, mappings, groups, held)
	var errs []error
	for _, g := range set {
		if err := s.d.Roles.Grant(ctx, g.Target, user, g.Role); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, s.d.Grants.Save(ctx, g), s.d.Audit.Record(ctx, user, "sso.role_granted", g.Target, g.Role))
	}
	for _, g := range revoke {
		if err := s.d.Roles.Revoke(ctx, g.Target, user); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, s.d.Grants.Remove(ctx, user, g.Target), s.d.Audit.Record(ctx, user, "sso.role_revoked", g.Target, g.Role))
	}
	return errors.Join(errs...)
}
