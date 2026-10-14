package application

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// ErrDirectoryDisabled: the server was started without SCIM provisioning.
var ErrDirectoryDisabled = errors.New("SCIM provisioning is not enabled")

// WithDirectory enables SCIM 2.0 user provisioning (ADR-0017).
func WithDirectory(d domain.Directory) Option { return func(s *Service) { s.dir = d } }

// ProvisionUser creates a user from the identity provider. It has no password: the user signs in
// through single sign-on.
func (s *Service) ProvisionUser(ctx context.Context, rawEmail, name string, active bool) (*domain.User, error) {
	if s.dir == nil {
		return nil, ErrDirectoryDisabled
	}
	email, err := domain.ParseEmail(rawEmail)
	if err != nil {
		return nil, err
	}
	u, err := domain.NewUser(domain.UserID(s.newID()), email, name, "")
	if err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	if !active {
		u.Deactivate()
		return u, s.dir.SaveUser(ctx, u)
	}
	return u, nil
}

// DirectoryUser returns one user for the identity provider.
func (s *Service) DirectoryUser(ctx context.Context, id string) (*domain.User, error) {
	if s.dir == nil {
		return nil, ErrDirectoryDisabled
	}
	return s.users.ByID(ctx, domain.UserID(id))
}

// DirectoryUsers pages through users (email "" = all).
func (s *Service) DirectoryUsers(ctx context.Context, rawEmail string, offset, limit int) ([]*domain.User, int, error) {
	if s.dir == nil {
		return nil, 0, ErrDirectoryDisabled
	}
	var email domain.Email
	if rawEmail != "" {
		e, err := domain.ParseEmail(rawEmail)
		if err != nil {
			return nil, 0, nil // an impossible address matches nobody
		}
		email = e
	}
	return s.dir.ListUsers(ctx, email, offset, limit)
}

// DirectoryChange is a partial update from the identity provider; nil leaves a field alone.
type DirectoryChange struct {
	Name   *string
	Active *bool
}

// UpdateDirectoryUser renames, deactivates or reactivates a user. Deactivation signs them out
// everywhere; their personal access tokens stop working while they are inactive.
func (s *Service) UpdateDirectoryUser(ctx context.Context, id string, c DirectoryChange) (*domain.User, error) {
	if s.dir == nil {
		return nil, ErrDirectoryDisabled
	}
	u, err := s.users.ByID(ctx, domain.UserID(id))
	if err != nil {
		return nil, err
	}
	if c.Name != nil {
		if err := u.Rename(*c.Name); err != nil {
			return nil, err
		}
	}
	if c.Active != nil {
		if *c.Active {
			u.Activate()
		} else {
			u.Deactivate()
		}
	}
	if err := s.dir.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	if !u.Active() {
		if err := s.dir.DeleteSessionsOf(ctx, u.ID()); err != nil {
			return nil, err
		}
	}
	return u, nil
}
