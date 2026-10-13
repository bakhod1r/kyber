package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

// Workflows stores workflows in memory.
type Workflows struct {
	mu  sync.Mutex
	all []domain.Workflow
}

func NewWorkflows() *Workflows { return &Workflows{} }

func (s *Workflows) Add(_ context.Context, w *domain.Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = append(s.all, *w)
	return nil
}

func (s *Workflows) Remove(_ context.Context, id domain.WorkflowID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = slices.DeleteFunc(s.all, func(w domain.Workflow) bool { return w.ID == id })
	return nil
}

func (s *Workflows) ByID(_ context.Context, id domain.WorkflowID) (*domain.Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.all {
		if w.ID == id {
			return &w, nil
		}
	}
	return nil, domain.ErrWorkflowNotFound
}

func (s *Workflows) list(keep func(domain.Workflow) bool) []*domain.Workflow {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Workflow{}
	for _, w := range s.all {
		if keep(w) {
			c := w
			out = append(out, &c)
		}
	}
	return out
}

func (s *Workflows) In(_ context.Context, conv domain.ConversationID) ([]*domain.Workflow, error) {
	return s.list(func(w domain.Workflow) bool { return w.Conversation == conv }), nil
}

func (s *Workflows) Scheduled(context.Context) ([]*domain.Workflow, error) {
	return s.list(func(w domain.Workflow) bool { return w.Trigger == domain.OnSchedule }), nil
}

func (s *Workflows) MarkRan(_ context.Context, w *domain.Workflow, prev time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.all {
		if s.all[i].ID == w.ID && s.all[i].LastRun.Equal(prev) {
			s.all[i].LastRun = w.LastRun
			return true, nil
		}
	}
	return false, nil
}
