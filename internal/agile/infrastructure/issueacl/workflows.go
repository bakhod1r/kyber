// Package issueacl is the anti-corruption layer from Agile to Issue Tracking's workflows.
package issueacl

import (
	"context"

	agileapp "github.com/bakhod1r/kyber/internal/agile/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
)

type Source interface {
	ProjectWorkflow(ctx context.Context, project string) (*issuedomain.Workflow, error)
}

type Workflows struct{ src Source }

func New(src Source) Workflows { return Workflows{src: src} }

func (w Workflows) Statuses(ctx context.Context, project string) ([]agileapp.WorkflowStatus, error) {
	wf, err := w.src.ProjectWorkflow(ctx, project)
	if err != nil {
		return nil, err
	}
	out := []agileapp.WorkflowStatus{}
	for _, s := range wf.Statuses() {
		out = append(out, agileapp.WorkflowStatus{ID: string(s.ID), Name: s.Name})
	}
	return out, nil
}
