// Package memory is an in-memory sprint repository for tests and dev mode.
package memory

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"sort"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

type entry struct {
	snap domain.Snapshot
	seq  int    // creation order
	ws   string // workspace (ADR-0004 phase 2)
}

type Repository struct {
	mu     sync.Mutex
	byID   map[domain.SprintID]entry
	seq    int
	outbox []domain.Event
	outAt  []outbox.Meta // when and in which workspace each event was recorded
}

func NewRepository() *Repository { return &Repository{byID: map[domain.SprintID]entry{}} }

func snapshot(s *domain.Sprint) domain.Snapshot {
	return domain.Snapshot{ID: s.ID(), Project: s.Project(), Name: s.Name(), Goal: s.Goal(), State: s.State(),
		StartedAt: s.StartedAt(), EndsAt: s.EndsAt(), CompletedAt: s.CompletedAt(), Version: s.Version()}
}

func (r *Repository) Save(ctx context.Context, s *domain.Sprint, events []domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.State() == domain.StateActive {
		for id, e := range r.byID {
			if id != s.ID() && e.ws == tenant.Workspace(ctx) && e.snap.Project == s.Project() && e.snap.State == domain.StateActive {
				return domain.ErrAnotherSprintLive
			}
		}
	}
	e, ok := r.byID[s.ID()]
	if ok && e.ws != tenant.Workspace(ctx) {
		return domain.ErrSprintConflict // another workspace's sprint
	}
	if ok != (s.Version() > 0) || (ok && e.snap.Version != s.Version()) {
		return domain.ErrSprintConflict
	}
	if !ok {
		r.seq++
		e.seq, e.ws = r.seq, tenant.Workspace(ctx)
	}
	s.MarkPersisted()
	e.snap = snapshot(s)
	r.byID[s.ID()] = e
	r.outbox, r.outAt = append(r.outbox, events...), appendNow(ctx, r.outAt, len(events))
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.SprintID) (*domain.Sprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok || e.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrSprintNotFound
	}
	return domain.Rehydrate(e.snap), nil
}

func stateOrder(s domain.State) int {
	switch s {
	case domain.StateActive:
		return 0
	case domain.StatePlanned:
		return 1
	}
	return 2
}

func (r *Repository) ListByProject(ctx context.Context, project string) ([]*domain.Sprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []entry
	for _, e := range r.byID {
		if e.ws == tenant.Workspace(ctx) && e.snap.Project == project {
			list = append(list, e)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if stateOrder(a.snap.State) != stateOrder(b.snap.State) {
			return stateOrder(a.snap.State) < stateOrder(b.snap.State)
		}
		if a.snap.State == domain.StateClosed {
			return a.snap.CompletedAt.After(b.snap.CompletedAt)
		}
		return a.seq < b.seq
	})
	out := make([]*domain.Sprint, len(list))
	for i, e := range list {
		out[i] = domain.Rehydrate(e.snap)
	}
	return out, nil
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
