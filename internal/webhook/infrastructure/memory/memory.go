// Package memory stores webhooks, their deliveries and development links in memory (dev mode and tests).
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

// Store keeps all three repositories' data, so deleting a webhook also drops its deliveries.
type Store struct {
	mu         sync.Mutex
	webhooks   map[string]map[domain.ID]domain.Webhook // workspace → id
	order      []domain.ID
	deliveries []domain.Delivery
	links      map[string][]domain.Link // workspace → links
}

func NewStore() *Store {
	return &Store{webhooks: map[string]map[domain.ID]domain.Webhook{}, links: map[string][]domain.Link{}}
}

// Webhooks, Deliveries and Links are the store's repositories.
func (s *Store) Webhooks() domain.Repository           { return webhooks{s} }
func (s *Store) Deliveries() domain.DeliveryRepository { return deliveries{s} }
func (s *Store) Links() domain.LinkRepository          { return links{s} }

type webhooks struct{ s *Store }

func clone(w domain.Webhook) *domain.Webhook {
	w.Events = slices.Clone(w.Events)
	return &w
}

func (r webhooks) Save(ctx context.Context, w *domain.Webhook) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	ws := tenant.Workspace(ctx)
	if r.s.webhooks[ws] == nil {
		r.s.webhooks[ws] = map[domain.ID]domain.Webhook{}
	}
	if _, ok := r.s.webhooks[ws][w.ID]; !ok {
		r.s.order = append(r.s.order, w.ID)
	}
	r.s.webhooks[ws][w.ID] = *clone(*w)
	return nil
}

func (r webhooks) ByID(ctx context.Context, id domain.ID) (*domain.Webhook, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	w, ok := r.s.webhooks[tenant.Workspace(ctx)][id]
	if !ok {
		return nil, domain.ErrWebhookNotFound
	}
	return clone(w), nil
}

func (r webhooks) ForProject(ctx context.Context, project string) ([]*domain.Webhook, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []*domain.Webhook
	for _, id := range r.s.order {
		if w, ok := r.s.webhooks[tenant.Workspace(ctx)][id]; ok && w.Project == project {
			out = append(out, clone(w))
		}
	}
	return out, nil
}

func (r webhooks) Delete(ctx context.Context, id domain.ID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	ws := tenant.Workspace(ctx)
	if _, ok := r.s.webhooks[ws][id]; !ok {
		return domain.ErrWebhookNotFound
	}
	delete(r.s.webhooks[ws], id)
	r.s.deliveries = slices.DeleteFunc(r.s.deliveries, func(d domain.Delivery) bool { return d.Workspace == ws && d.Webhook == id })
	return nil
}

type deliveries struct{ s *Store }

func (r deliveries) Enqueue(_ context.Context, d domain.Delivery) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if !slices.ContainsFunc(r.s.deliveries, func(o domain.Delivery) bool { return o.Key == d.Key }) {
		r.s.deliveries = append(r.s.deliveries, d)
	}
	return nil
}

func (r deliveries) Claim(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Delivery
	for i, d := range r.s.deliveries {
		if d.Status == domain.DeliveryPending && !d.NextAt.After(now) && len(out) < limit {
			r.s.deliveries[i].NextAt = now.Add(lease)
			d.NextAt = now
			out = append(out, d)
		}
	}
	slices.SortStableFunc(out, func(a, b domain.Delivery) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, nil
}

func (r deliveries) Save(_ context.Context, d domain.Delivery) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if i := slices.IndexFunc(r.s.deliveries, func(o domain.Delivery) bool { return o.ID == d.ID }); i >= 0 {
		r.s.deliveries[i] = d
	}
	return nil
}

func (r deliveries) Recent(ctx context.Context, webhook domain.ID, limit int) ([]domain.Delivery, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Delivery
	for _, d := range slices.Backward(r.s.deliveries) {
		if d.Workspace == tenant.Workspace(ctx) && d.Webhook == webhook && len(out) < limit {
			out = append(out, d)
		}
	}
	return out, nil
}

type links struct{ s *Store }

func (r links) Save(ctx context.Context, l domain.Link) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	ws := tenant.Workspace(ctx)
	list := slices.DeleteFunc(r.s.links[ws], func(o domain.Link) bool {
		return o.Issue == l.Issue && o.Kind == l.Kind && o.Repo == l.Repo && o.Ref == l.Ref
	})
	r.s.links[ws] = append(list, l)
	return nil
}

func (r links) ForIssue(ctx context.Context, issue string) ([]domain.Link, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Link
	for _, l := range r.s.links[tenant.Workspace(ctx)] {
		if l.Issue == issue {
			out = append(out, l)
		}
	}
	slices.SortStableFunc(out, func(a, b domain.Link) int { return cmp.Compare(b.UpdatedAt.UnixNano(), a.UpdatedAt.UnixNano()) })
	return out, nil
}
