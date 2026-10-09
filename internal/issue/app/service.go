// Package app holds the Issue Tracking context use cases.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidKey      = errors.New("invalid issue key")
	ErrForbidden       = errors.New("insufficient project role")
)

// Access is the anti-corruption port to the Project context's membership rules.
// Non-members get ErrProjectNotFound, members lacking the permission ErrForbidden.
type Access interface {
	Authorize(ctx context.Context, actor, project string, write bool) error
}

type Service struct {
	issues   domain.Repository
	keys     domain.KeyAllocator
	access   Access
	workflow *domain.Workflow
	newID    func() string
}

func NewService(issues domain.Repository, keys domain.KeyAllocator, access Access,
	wf *domain.Workflow, newID func() string) *Service {
	return &Service{issues: issues, keys: keys, access: access, workflow: wf, newID: newID}
}

type CreateIssue struct {
	Project string
	Title   string
	Type    string
}

func (s *Service) Create(ctx context.Context, actor string, cmd CreateIssue) (*domain.Issue, error) {
	typ, err := domain.ParseIssueType(cmd.Type)
	if err != nil {
		return nil, err
	}
	// Validate before allocating so rejected commands do not burn issue numbers.
	if _, err := domain.NormalizeTitle(cmd.Title); err != nil {
		return nil, err
	}
	if err := s.access.Authorize(ctx, actor, cmd.Project, true); err != nil {
		return nil, err
	}
	key, err := s.keys.Next(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	is, err := domain.NewIssue(domain.IssueID(s.newID()), key, cmd.Title, typ, s.workflow)
	if err != nil {
		return nil, err
	}
	return is, s.save(ctx, is)
}

func (s *Service) Get(ctx context.Context, actor, rawKey string) (*domain.Issue, error) {
	return s.load(ctx, actor, rawKey, false)
}

// load parses the key and authorizes before touching the issue, so outsiders
// cannot distinguish missing issues from hidden ones.
func (s *Service) load(ctx context.Context, actor, rawKey string, write bool) (*domain.Issue, error) {
	key, err := domain.ParseIssueKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if err := s.access.Authorize(ctx, actor, key.Project(), write); err != nil {
		return nil, err
	}
	return s.issues.ByKey(ctx, key)
}

func (s *Service) Transition(ctx context.Context, actor, rawKey, to string) (*domain.Issue, error) {
	is, err := s.load(ctx, actor, rawKey, true)
	if err != nil {
		return nil, err
	}
	if err := is.Transition(domain.StatusID(to), s.workflow); err != nil {
		return nil, err
	}
	return is, s.save(ctx, is)
}

func (s *Service) List(ctx context.Context, actor, project, status string) ([]*domain.Issue, error) {
	if err := s.access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	var filter *domain.StatusID
	if status != "" {
		st := domain.StatusID(status)
		filter = &st
	}
	return s.issues.ListByProject(ctx, project, filter)
}

func (s *Service) save(ctx context.Context, is *domain.Issue) error {
	// The repository writes the aggregate and its events atomically (transactional outbox).
	return s.issues.Save(ctx, is, is.PullEvents())
}
