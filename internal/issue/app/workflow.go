package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// workflowFor returns the project's configured workflow, or the default one.
func (s *Service) workflowFor(ctx context.Context, project string) (*domain.Workflow, error) {
	def, err := s.workflows.ByProject(ctx, project)
	if errors.Is(err, domain.ErrWorkflowNotFound) {
		return s.workflow, nil
	}
	if err != nil {
		return nil, err
	}
	return domain.DefineWorkflow(def)
}

// Workflow returns the project's workflow to any project member.
func (s *Service) Workflow(ctx context.Context, actor, project string) (*domain.Workflow, error) {
	if err := s.access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	return s.workflowFor(ctx, project)
}

// SetWorkflow replaces the project's workflow (project admins only). Like Jira, a status that
// issues still sit in cannot be removed until they are moved.
func (s *Service) SetWorkflow(ctx context.Context, actor, project string, def domain.WorkflowDef) (*domain.Workflow, error) {
	if err := s.admins.Administer(ctx, actor, project); err != nil {
		return nil, err
	}
	wf, err := domain.DefineWorkflow(def)
	if err != nil {
		return nil, err
	}
	issues, err := s.issues.ListByProject(ctx, project, domain.ListFilter{})
	if err != nil {
		return nil, err
	}
	for _, is := range issues {
		if !wf.Has(is.Status()) {
			return nil, fmt.Errorf("%s (%s): %w", is.Status(), is.Key(), ErrStatusInUse)
		}
	}
	return wf, s.workflows.Save(ctx, project, def)
}
