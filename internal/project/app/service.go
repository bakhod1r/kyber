// Package app holds the Project context use cases.
package app

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

var ErrUnknownUser = errors.New("no user with that email")

// UserInfo is the Identity context's view of a user, translated by the directory adapter.
type UserInfo struct {
	ID    domain.UserID
	Email string
	Name  string
}

// UserDirectory is the anti-corruption port to the Identity context.
type UserDirectory interface {
	ByEmail(ctx context.Context, email string) (UserInfo, error) // ErrUnknownUser
	ByID(ctx context.Context, id domain.UserID) (UserInfo, error)
}

type Service struct {
	repo  domain.Repository
	users UserDirectory
	newID func() string
}

func NewService(repo domain.Repository, users UserDirectory, newID func() string) *Service {
	return &Service{repo: repo, users: users, newID: newID}
}

func (s *Service) Create(ctx context.Context, actor domain.UserID, key, name string) (*domain.Project, error) {
	p, err := domain.NewProject(domain.ProjectID(s.newID()), key, name, actor)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, actor domain.UserID, key string) (*domain.Project, error) {
	p, err := s.repo.ByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := p.Authorize(actor, domain.PermRead); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, actor domain.UserID) ([]*domain.Project, error) {
	return s.repo.ListForUser(ctx, actor)
}

// Authorize checks the actor's permission on a project (used by other contexts via ACL).
func (s *Service) Authorize(ctx context.Context, actor domain.UserID, key string, need domain.Permission) error {
	p, err := s.repo.ByKey(ctx, key)
	if err != nil {
		return err
	}
	return p.Authorize(actor, need)
}

// SetMember adds a user (by email) or changes their role; admin only.
func (s *Service) SetMember(ctx context.Context, actor domain.UserID, key, email, rawRole string) error {
	role, err := domain.ParseRole(rawRole)
	if err != nil {
		return err
	}
	if err := s.Authorize(ctx, actor, key, domain.PermAdmin); err != nil {
		return err
	}
	u, err := s.users.ByEmail(ctx, email)
	if err != nil {
		return err
	}
	return s.repo.Update(ctx, key, func(p *domain.Project) error {
		if err := p.Authorize(actor, domain.PermAdmin); err != nil { // re-check under lock
			return err
		}
		return p.SetMember(u.ID, role)
	})
}

type MemberInfo struct {
	UserInfo
	Role domain.Role
}

func (s *Service) Members(ctx context.Context, actor domain.UserID, key string) ([]MemberInfo, error) {
	p, err := s.Get(ctx, actor, key)
	if err != nil {
		return nil, err
	}
	out := make([]MemberInfo, 0, len(p.Members()))
	for _, m := range p.Members() {
		u, err := s.users.ByID(ctx, m.UserID)
		if err != nil {
			return nil, err
		}
		out = append(out, MemberInfo{UserInfo: u, Role: m.Role})
	}
	return out, nil
}

// NextIssueNumber atomically advances the project's issue sequence.
func (s *Service) NextIssueNumber(ctx context.Context, key string) (int, error) {
	var n int
	err := s.repo.Update(ctx, key, func(p *domain.Project) error {
		n = p.NextIssueNumber()
		return nil
	})
	return n, err
}
