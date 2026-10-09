// Package memory is an in-memory notification store for tests and dev mode.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type key struct {
	event     int64
	recipient domain.UserID
}

type Repository struct {
	mu   sync.Mutex
	all  []domain.Notification
	seen map[key]bool
}

func NewRepository() *Repository { return &Repository{seen: map[key]bool{}} }

func (r *Repository) Add(_ context.Context, n domain.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key{n.SourceEvent, n.Recipient}
	if r.seen[k] {
		return nil
	}
	r.seen[k] = true
	r.all = append(r.all, n)
	return nil
}

func (r *Repository) ListFor(_ context.Context, in domain.Inbox, unreadOnly bool, limit int) ([]domain.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Notification
	for _, n := range r.all {
		if in.Holds(n) && (!unreadOnly || !n.Read()) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) UnreadCount(_ context.Context, in domain.Inbox) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := 0
	for _, n := range r.all {
		if in.Holds(n) && !n.Read() {
			c++
		}
	}
	return c, nil
}

func (r *Repository) MarkRead(_ context.Context, in domain.Inbox, id domain.NotificationID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.all {
		if r.all[i].ID == id && in.Holds(r.all[i]) {
			if !r.all[i].Read() {
				r.all[i].ReadAt = at
			}
			return nil
		}
	}
	return domain.ErrNotificationNotFound
}

func (r *Repository) MarkAllRead(_ context.Context, in domain.Inbox, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.all {
		if in.Holds(r.all[i]) && !r.all[i].Read() {
			r.all[i].ReadAt = at
		}
	}
	return nil
}
