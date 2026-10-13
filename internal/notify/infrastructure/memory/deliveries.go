package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// DeliveryRepository is an in-memory delivery queue.
type DeliveryRepository struct {
	mu    sync.Mutex
	byID  map[string]domain.Delivery
	keys  map[string]bool
	order []string
}

func NewDeliveryRepository() *DeliveryRepository {
	return &DeliveryRepository{byID: map[string]domain.Delivery{}, keys: map[string]bool{}}
}

func (r *DeliveryRepository) Enqueue(_ context.Context, d domain.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.keys[d.Key] {
		return nil
	}
	r.keys[d.Key] = true
	r.byID[d.ID] = d
	r.order = append(r.order, d.ID)
	return nil
}

func (r *DeliveryRepository) Claim(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var due []domain.Delivery
	for _, id := range r.order {
		if d := r.byID[id]; d.Status == domain.DeliveryPending && !d.NextAt.After(now) {
			due = append(due, d)
		}
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].NextAt.Before(due[j].NextAt) })
	if len(due) > limit {
		due = due[:limit]
	}
	for _, d := range due {
		held := r.byID[d.ID]
		held.NextAt = now.Add(lease)
		r.byID[d.ID] = held
	}
	return due, nil
}

func (r *DeliveryRepository) Save(_ context.Context, d domain.Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[d.ID] = d
	return nil
}

// All returns every delivery in enqueue order (tests and diagnostics).
func (r *DeliveryRepository) All() []domain.Delivery {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Delivery, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}
