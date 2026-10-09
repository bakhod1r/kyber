// Package app holds the Project context use cases.
package app

import (
	"context"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Service struct {
	repo  domain.Repository
	newID func() string
}

func NewService(repo domain.Repository, newID func() string) *Service {
	return &Service{repo: repo, newID: newID}
}

func (s *Service) Create(ctx context.Context, key, name string) (*domain.Project, error) {
	p, err := domain.NewProject(domain.ProjectID(s.newID()), key, name)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Get(ctx context.Context, key string) (*domain.Project, error) {
	return s.repo.ByKey(ctx, key)
}

func (s *Service) List(ctx context.Context) ([]*domain.Project, error) {
	return s.repo.List(ctx)
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
