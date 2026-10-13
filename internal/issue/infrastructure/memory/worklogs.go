package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// WorklogRepository keeps the worklog projection in memory.
type WorklogRepository struct {
	mu   sync.Mutex
	byID map[domain.WorklogID]domain.Worklog
}

func NewWorklogRepository() *WorklogRepository {
	return &WorklogRepository{byID: map[domain.WorklogID]domain.Worklog{}}
}

func (r *WorklogRepository) Add(_ context.Context, w domain.Worklog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[w.ID] = w
	return nil
}

func (r *WorklogRepository) Remove(_ context.Context, id domain.WorklogID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

func (r *WorklogRepository) ByID(_ context.Context, id domain.WorklogID) (domain.Worklog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.byID[id]
	if !ok {
		return domain.Worklog{}, domain.ErrWorklogNotFound
	}
	return w, nil
}

func (r *WorklogRepository) ForIssue(_ context.Context, issue domain.IssueID) ([]domain.Worklog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Worklog
	for _, w := range r.byID {
		if w.Issue == issue {
			out = append(out, w)
		}
	}
	slices.SortFunc(out, func(a, b domain.Worklog) int {
		if c := a.Started.Compare(b.Started); c != 0 {
			return c
		}
		return slices.Compare([]byte(a.ID), []byte(b.ID))
	})
	return out, nil
}
