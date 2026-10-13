// Package memory keeps announcements in memory (tests, dev mode); their events feed the outbox relay.
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct {
	mu     sync.Mutex
	byID   map[domain.AnnouncementID]domain.Announcement
	outbox []domain.Event
	outAt  []outbox.Meta
}

func NewRepository() *Repository {
	return &Repository{byID: map[domain.AnnouncementID]domain.Announcement{}}
}

func (r *Repository) Save(ctx context.Context, a *domain.Announcement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := a.PullEvents()
	r.byID[a.ID] = *a
	m := outbox.MetaOf(ctx, time.Now())
	for _, e := range events {
		r.outbox = append(r.outbox, e)
		r.outAt = append(r.outAt, m)
	}
	return nil
}

func (r *Repository) ByID(ctx context.Context, id domain.AnnouncementID) (*domain.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok || a.Workspace != tenant.Workspace(ctx) {
		return nil, domain.ErrAnnouncementNotFound
	}
	return &a, nil
}

func (r *Repository) For(ctx context.Context, project string, limit int) ([]*domain.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Announcement
	for _, a := range r.byID {
		if a.Workspace == tenant.Workspace(ctx) && a.Project == project {
			out = append(out, &a)
		}
	}
	slices.SortFunc(out, func(a, b *domain.Announcement) int { return cmp.Or(b.At.Compare(a.At), cmp.Compare(a.ID, b.ID)) })
	return out[:min(limit, len(out))], nil
}

// Outbox is the append-only event log the relay reads.
func (r *Repository) Outbox() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outbox)
}

// OutboxMeta is the envelope of Outbox(), index by index.
func (r *Repository) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.outAt)
}
