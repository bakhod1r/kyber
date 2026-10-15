// Package presence knows who is online on any replica (ADR-0013 amendment, S105). Each replica
// periodically writes the set of users connected to it (a beat); a user is online while a seat was
// seen recently. A crashed replica stops beating, so its seats simply age out.
package presence

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Seat is one user connected in one workspace.
type Seat struct{ Workspace, User string }

// Store keeps the replicas' seats.
type Store interface {
	// Beat replaces replica's seats with seats, all seen at.
	Beat(ctx context.Context, replica string, seats []Seat, at time.Time) error
	// Online lists which of users have a seat in ws seen after since (request order, no duplicates).
	Online(ctx context.Context, ws string, users []string, since time.Time) ([]string, error)
	// Expire deletes seats last seen at or before before, returning how many.
	Expire(ctx context.Context, before time.Time) (int, error)
}

// Timing of the roster: a beat every Every; a seat counts for TTL (three beats), so one lost beat
// does not flicker anyone offline.
const (
	Every = 15 * time.Second
	TTL   = 3 * Every
)

// Local lists the seats on this replica (MQTT clients and SSE streams).
type Local func() []Seat

// Roster is this replica's view: it beats its local seats into the store and answers who is online.
type Roster struct {
	store   Store
	replica string
	local   Local
	now     func() time.Time
	log     *slog.Logger
	kick    chan struct{}
	once    sync.Once
}

func NewRoster(store Store, replica string, local Local, now func() time.Time, log *slog.Logger) *Roster {
	return &Roster{store: store, replica: replica, local: local, now: now, log: log, kick: make(chan struct{}, 1)}
}

// Changed asks for a beat soon (someone connected or disconnected here).
func (r *Roster) Changed() {
	select {
	case r.kick <- struct{}{}:
	default:
	}
}

// Beat writes this replica's seats now.
func (r *Roster) Beat(ctx context.Context) error {
	return r.store.Beat(ctx, r.replica, r.local(), r.now())
}

// Run beats every Every and on Changed, expiring stale seats, until ctx ends; then it clears this
// replica's seats so a clean shutdown takes its users offline at once.
func (r *Roster) Run(ctx context.Context) {
	tick := time.NewTicker(Every)
	defer tick.Stop()
	for {
		if err := r.Beat(ctx); err != nil && ctx.Err() == nil {
			r.log.WarnContext(ctx, "presence beat", "err", err)
		}
		if _, err := r.store.Expire(ctx, r.now().Add(-TTL)); err != nil && ctx.Err() == nil {
			r.log.WarnContext(ctx, "presence expiry", "err", err)
		}
		select {
		case <-ctx.Done():
			r.once.Do(func() {
				c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = r.store.Beat(c, r.replica, nil, r.now())
			})
			return
		case <-tick.C:
		case <-r.kick:
		}
	}
}

// Online lists which of users are online in ws on any replica. This replica's own seats count at
// once, before its next beat reaches the store.
func (r *Roster) Online(ctx context.Context, ws string, users []string) ([]string, error) {
	on := map[string]bool{}
	for _, s := range r.local() {
		if s.Workspace == ws {
			on[s.User] = true
		}
	}
	shared, err := r.store.Online(ctx, ws, users, r.now().Add(-TTL))
	if err != nil {
		return nil, err
	}
	for _, u := range shared {
		on[u] = true
	}
	out := []string{}
	for _, u := range users {
		if on[u] && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	return out, nil
}
