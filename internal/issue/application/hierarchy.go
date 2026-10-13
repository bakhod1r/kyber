package application

import (
	"context"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// parentFor loads and checks the parent named by rawKey ("" = none) for an issue of type typ.
func (s *Service) parentFor(ctx context.Context, project, rawKey string, typ domain.IssueType) (*domain.Issue, error) {
	var parent *domain.Issue
	if rawKey != "" {
		key, err := domain.ParseIssueKey(rawKey)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
		}
		if key.Project() != project {
			return nil, fmt.Errorf("parent %s is in another project: %w", key, domain.ErrInvalidParent)
		}
		if parent, err = s.issues.ByKey(ctx, key); err != nil {
			return nil, err
		}
	}
	return parent, domain.CheckParent(project, typ, parent)
}

// Children lists an issue's children (an epic's issues, an issue's sub-tasks) with Jira's
// progress: how many, and how many story points, are in a Done-category status.
type Children struct {
	Items              []*domain.Issue
	Done               int
	Points, DonePoints int // tenths
}

func (s *Service) Children(ctx context.Context, actor, rawKey string) (Children, error) {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil {
		return Children{}, err
	}
	id := is.ID()
	items, err := s.issues.ListByProject(ctx, is.Key().Project(), domain.ListFilter{Parent: &id})
	if err != nil {
		return Children{}, err
	}
	if items, err = s.visible(ctx, actor, is.Key().Project(), items); err != nil {
		return Children{}, err
	}
	wf, err := s.workflowFor(ctx, is.Key().Project())
	if err != nil {
		return Children{}, err
	}
	out := Children{Items: items}
	for _, c := range items {
		p := 0
		if est, ok := c.Estimate(); ok {
			p = int(est)
		}
		out.Points += p
		if wf.IsDone(c.Status()) {
			out.Done++
			out.DonePoints += p
		}
	}
	if out.Items == nil {
		out.Items = []*domain.Issue{}
	}
	return out, nil
}
