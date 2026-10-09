// Package memory is an in-memory Issue repository for tests and local runs.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type Repository struct {
	mu     sync.Mutex
	byKey  map[string]domain.Issue
	outbox []domain.Event
}

func NewRepository() *Repository { return &Repository{byKey: map[string]domain.Issue{}} }

func (r *Repository) Save(_ context.Context, is *domain.Issue, events []domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, exists := r.byKey[is.Key().String()]
	switch {
	case is.Version() == 0 && exists,
		is.Version() > 0 && (!exists || stored.Version() != is.Version()):
		return domain.ErrConcurrentModification
	}
	is.MarkPersisted()
	r.byKey[is.Key().String()] = *is
	r.outbox = append(r.outbox, events...)
	return nil
}

// OutboxNames returns the names of all events written so far.
func (r *Repository) OutboxNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, len(r.outbox))
	for i, e := range r.outbox {
		names[i] = e.EventName()
	}
	return names
}

// Outbox returns all events written so far.
func (r *Repository) Outbox() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Event(nil), r.outbox...)
}

func (r *Repository) ByKey(_ context.Context, key domain.IssueKey) (*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	is, ok := r.byKey[key.String()]
	if !ok {
		return nil, domain.ErrIssueNotFound
	}
	return &is, nil
}

func (r *Repository) ListByProject(_ context.Context, project string, f domain.ListFilter) ([]*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Issue
	for _, is := range r.byKey {
		if is.Key().Project() != project ||
			(f.Status != nil && is.Status() != *f.Status) ||
			(f.Sprint != nil && is.Sprint() != *f.Sprint) {
			continue
		}
		out = append(out, &is)
	}
	sortByRank(out)
	return out, nil
}

func sortByRank(list []*domain.Issue) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Rank() != list[j].Rank() {
			return list[i].Rank() < list[j].Rank()
		}
		return list[i].Key().Number() < list[j].Key().Number()
	})
}

func (r *Repository) ranks(project string) []domain.Rank {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Rank
	for _, is := range r.byKey {
		if is.Key().Project() == project {
			out = append(out, is.Rank())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (r *Repository) LastRank(_ context.Context, project string) (domain.Rank, error) {
	rs := r.ranks(project)
	if len(rs) == 0 {
		return "", nil
	}
	return rs[len(rs)-1], nil
}

func (r *Repository) NextRank(_ context.Context, project string, after domain.Rank) (domain.Rank, error) {
	for _, x := range r.ranks(project) {
		if x > after {
			return x, nil
		}
	}
	return "", nil
}

func (r *Repository) PrevRank(_ context.Context, project string, before domain.Rank) (domain.Rank, error) {
	rs := r.ranks(project)
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] < before {
			return rs[i], nil
		}
	}
	return "", nil
}

func (r *Repository) appendOutbox(events []domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outbox = append(r.outbox, events...)
}
