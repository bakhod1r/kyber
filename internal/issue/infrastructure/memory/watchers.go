package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// WatcherRepository keeps watchers in memory (issue ids are globally unique).
type WatcherRepository struct {
	mu   sync.Mutex
	byIs map[domain.IssueID][]domain.UserID
}

func NewWatcherRepository() *WatcherRepository {
	return &WatcherRepository{byIs: map[domain.IssueID][]domain.UserID{}}
}

func (r *WatcherRepository) Watch(_ context.Context, issue domain.IssueID, user domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !slices.Contains(r.byIs[issue], user) {
		r.byIs[issue] = append(r.byIs[issue], user)
	}
	return nil
}

func (r *WatcherRepository) Unwatch(_ context.Context, issue domain.IssueID, user domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byIs[issue] = slices.DeleteFunc(r.byIs[issue], func(u domain.UserID) bool { return u == user })
	return nil
}

func (r *WatcherRepository) Watchers(_ context.Context, issue domain.IssueID) ([]domain.UserID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.byIs[issue]), nil
}
