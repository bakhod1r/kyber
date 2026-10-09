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
)

type Service struct {
	issues   domain.Repository
	keys     domain.KeyAllocator
	events   domain.EventPublisher
	workflow *domain.Workflow
	newID    func() string
}

func NewService(issues domain.Repository, keys domain.KeyAllocator, events domain.EventPublisher,
	wf *domain.Workflow, newID func() string) *Service {
	return &Service{issues: issues, keys: keys, events: events, workflow: wf, newID: newID}
}

type CreateIssue struct {
	Project string
	Title   string
	Type    string
}

func (s *Service) Create(ctx context.Context, cmd CreateIssue) (*domain.Issue, error) {
	typ, err := domain.ParseIssueType(cmd.Type)
	if err != nil {
		return nil, err
	}
	// Validate before allocating so rejected commands do not burn issue numbers.
	if _, err := domain.NormalizeTitle(cmd.Title); err != nil {
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

func (s *Service) Get(ctx context.Context, rawKey string) (*domain.Issue, error) {
	key, err := domain.ParseIssueKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	return s.issues.ByKey(ctx, key)
}

func (s *Service) Transition(ctx context.Context, rawKey, to string) (*domain.Issue, error) {
	is, err := s.Get(ctx, rawKey)
	if err != nil {
		return nil, err
	}
	if err := is.Transition(domain.StatusID(to), s.workflow); err != nil {
		return nil, err
	}
	return is, s.save(ctx, is)
}

func (s *Service) List(ctx context.Context, project, status string) ([]*domain.Issue, error) {
	var filter *domain.StatusID
	if status != "" {
		st := domain.StatusID(status)
		filter = &st
	}
	return s.issues.ListByProject(ctx, project, filter)
}

func (s *Service) save(ctx context.Context, is *domain.Issue) error {
	events := is.PullEvents()
	if err := s.issues.Save(ctx, is); err != nil {
		return err
	}
	// TODO(outbox): persist events in the same transaction as the aggregate (ADR-0008).
	return s.events.Publish(ctx, events...)
}
