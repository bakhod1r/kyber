package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// WorkflowRepository keeps project workflows in memory, keyed by workspace and project.
type WorkflowRepository struct {
	mu   sync.Mutex
	defs map[string]domain.WorkflowDef
}

func NewWorkflowRepository() *WorkflowRepository {
	return &WorkflowRepository{defs: map[string]domain.WorkflowDef{}}
}

func (r *WorkflowRepository) ByProject(ctx context.Context, project string) (domain.WorkflowDef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	def, ok := r.defs[slot(ctx, project)]
	if !ok {
		return domain.WorkflowDef{}, domain.ErrWorkflowNotFound
	}
	return clone(def), nil
}

func (r *WorkflowRepository) Save(ctx context.Context, project string, def domain.WorkflowDef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[slot(ctx, project)] = clone(def)
	return nil
}

func clone(d domain.WorkflowDef) domain.WorkflowDef {
	out := domain.WorkflowDef{Initial: d.Initial}
	if d.Statuses != nil {
		out.Statuses = append([]domain.Status{}, d.Statuses...)
	}
	if d.Transitions != nil {
		out.Transitions = append([]domain.Transition{}, d.Transitions...)
	}
	return out
}
