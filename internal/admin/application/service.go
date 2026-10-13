// Package application holds the Administration context's use cases: a workspace's administrators
// manage roles and look after its projects and stuck events.
package application

import (
	"context"

	"github.com/bakhod1r/kyber/internal/admin/domain"
)

// Ports to the other contexts (anti-corruption layers live in infrastructure). All are scoped to
// the request's workspace.
type (
	// Gate: owners and admins of a named workspace; configured administrators of the default one.
	Gate interface {
		IsAdmin(ctx context.Context, actor string) (bool, error)
	}
	Directory interface {
		Members(ctx context.Context) ([]domain.Member, error)
		ChangeRole(ctx context.Context, user, role string) error
	}
	Projects interface {
		Projects(ctx context.Context) ([]domain.Project, error)
	}
	DeadLetters interface {
		DeadLetters(ctx context.Context) ([]domain.DeadLetter, error)
		Retry(ctx context.Context, id int64) error
	}
)

type Deps struct {
	Gate        Gate
	Directory   Directory
	Projects    Projects
	DeadLetters DeadLetters
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Access reports whether the actor administers the request's workspace (the UI shows the panel).
func (s *Service) Access(ctx context.Context, actor string) (bool, error) {
	ok, err := s.d.Gate.IsAdmin(ctx, actor)
	return ok && err == nil, err
}

func (s *Service) guard(ctx context.Context, actor string) error {
	ok, err := s.d.Gate.IsAdmin(ctx, actor)
	if err == nil && !ok {
		err = domain.ErrForbidden
	}
	return err
}

func (s *Service) Members(ctx context.Context, actor string) ([]domain.Member, error) {
	if err := s.guard(ctx, actor); err != nil {
		return nil, err
	}
	return s.d.Directory.Members(ctx)
}

func (s *Service) ChangeRole(ctx context.Context, actor, user, role string) error {
	if err := s.guard(ctx, actor); err != nil {
		return err
	}
	return s.d.Directory.ChangeRole(ctx, user, role)
}

func (s *Service) Projects(ctx context.Context, actor string) ([]domain.Project, error) {
	if err := s.guard(ctx, actor); err != nil {
		return nil, err
	}
	return s.d.Projects.Projects(ctx)
}

func (s *Service) DeadLetters(ctx context.Context, actor string) ([]domain.DeadLetter, error) {
	if err := s.guard(ctx, actor); err != nil {
		return nil, err
	}
	return s.d.DeadLetters.DeadLetters(ctx)
}

// Retry returns a dead letter to the queue once its handler is fixed.
func (s *Service) Retry(ctx context.Context, actor string, id int64) error {
	if err := s.guard(ctx, actor); err != nil {
		return err
	}
	return s.d.DeadLetters.Retry(ctx, id)
}
