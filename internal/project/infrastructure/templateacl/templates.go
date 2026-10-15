// Package templateacl sets up a new project from a template through the contexts that own its workflow (Issue
// Tracking) and its board (Agile) — the Project context's anti-corruption layer for B10.
package templateacl

import (
	"context"

	agiledomain "github.com/bakhod1r/kyber/internal/agile/domain"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

// Workflows is the part of Issue Tracking that replaces a project's workflow.
type Workflows interface {
	SetWorkflow(ctx context.Context, actor, project string, def issuedomain.WorkflowDef) (*issuedomain.Workflow, error)
}

// Boards is the part of Agile that picks the kind of a project's first board.
type Boards interface {
	UseBoardKind(ctx context.Context, actor, project string, kind agiledomain.BoardKind) error
}

type Templates struct {
	workflows Workflows
	boards    Boards
}

func New(w Workflows, b Boards) Templates { return Templates{workflows: w, boards: b} }

func (t Templates) Apply(ctx context.Context, actor domain.UserID, key string, tpl domain.Template) error {
	switch tpl {
	case domain.TemplateScrum:
		return t.boards.UseBoardKind(ctx, string(actor), key, agiledomain.KindScrum)
	case domain.TemplateBugTracking:
		_, err := t.workflows.SetWorkflow(ctx, string(actor), key, issuedomain.BugTrackingWorkflowDef())
		return err
	}
	return nil
}
