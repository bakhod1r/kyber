// Package memory is an in-memory Issue repository for tests and local runs.
package memory

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type Repository struct {
	mu     sync.Mutex
	byKey  map[string]domain.Issue
	outbox []domain.Event
	outAt  []outbox.Meta // when and in which workspace each event was recorded
}

func NewRepository() *Repository { return &Repository{byKey: map[string]domain.Issue{}} }

func (r *Repository) Save(ctx context.Context, is *domain.Issue, events []domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := slot(ctx, is.Key().String())
	stored, exists := r.byKey[k]
	switch {
	case is.Version() == 0 && exists,
		is.Version() > 0 && (!exists || stored.Version() != is.Version()):
		return domain.ErrConcurrentModification
	}
	is.MarkPersisted()
	r.byKey[k] = *is
	r.outbox, r.outAt = append(r.outbox, events...), appendNow(ctx, r.outAt, len(events))
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

// slot is the map key: issue keys are unique per workspace (ADR-0004 phase 2).
func slot(ctx context.Context, key string) string { return tenant.Workspace(ctx) + "/" + key }

func inWorkspace(ctx context.Context, k string) bool {
	return strings.HasPrefix(k, tenant.Workspace(ctx)+"/")
}

func (r *Repository) ByKey(ctx context.Context, key domain.IssueKey) (*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	is, ok := r.byKey[slot(ctx, key.String())]
	if !ok {
		return nil, domain.ErrIssueNotFound
	}
	return &is, nil
}

func (r *Repository) ListByProject(ctx context.Context, project string, f domain.ListFilter) ([]*domain.Issue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Issue
	for k, is := range r.byKey {
		if !inWorkspace(ctx, k) || is.Key().Project() != project ||
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

func (r *Repository) ranks(ctx context.Context, project string) []domain.Rank {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Rank
	for k, is := range r.byKey {
		if inWorkspace(ctx, k) && is.Key().Project() == project {
			out = append(out, is.Rank())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (r *Repository) LastRank(ctx context.Context, project string) (domain.Rank, error) {
	rs := r.ranks(ctx, project)
	if len(rs) == 0 {
		return "", nil
	}
	return rs[len(rs)-1], nil
}

func (r *Repository) NextRank(ctx context.Context, project string, after domain.Rank) (domain.Rank, error) {
	for _, x := range r.ranks(ctx, project) {
		if x > after {
			return x, nil
		}
	}
	return "", nil
}

func (r *Repository) PrevRank(ctx context.Context, project string, before domain.Rank) (domain.Rank, error) {
	rs := r.ranks(ctx, project)
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] < before {
			return rs[i], nil
		}
	}
	return "", nil
}

func (r *Repository) appendOutbox(ctx context.Context, events []domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outbox, r.outAt = append(r.outbox, events...), appendNow(ctx, r.outAt, len(events))
}

func appendNow(ctx context.Context, ts []outbox.Meta, n int) []outbox.Meta {
	m := outbox.Meta{At: time.Now(), Workspace: tenant.Workspace(ctx)}
	for range n {
		ts = append(ts, m)
	}
	return ts
}

// OutboxMeta is the recording time and workspace of Outbox(), index by index.
func (r *Repository) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]outbox.Meta(nil), r.outAt...)
}
