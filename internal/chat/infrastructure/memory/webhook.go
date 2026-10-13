package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

// Webhooks stores incoming webhooks in memory.
type Webhooks struct {
	mu  sync.Mutex
	all []domain.Webhook
}

func NewWebhooks() *Webhooks { return &Webhooks{} }

func (s *Webhooks) Add(_ context.Context, h *domain.Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = append(s.all, *h)
	return nil
}

func (s *Webhooks) Update(_ context.Context, h *domain.Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.all {
		if s.all[i].ID == h.ID {
			s.all[i].SecretHash = h.SecretHash
			return nil
		}
	}
	return domain.ErrWebhookNotFound
}

func (s *Webhooks) Remove(_ context.Context, id domain.WebhookID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = slices.DeleteFunc(s.all, func(h domain.Webhook) bool { return h.ID == id })
	return nil
}

func (s *Webhooks) find(keep func(domain.Webhook) bool) (*domain.Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.all {
		if keep(h) {
			return &h, nil
		}
	}
	return nil, domain.ErrWebhookNotFound
}

func (s *Webhooks) ByID(_ context.Context, id domain.WebhookID) (*domain.Webhook, error) {
	return s.find(func(h domain.Webhook) bool { return h.ID == id })
}

func (s *Webhooks) BySecret(_ context.Context, hash string) (*domain.Webhook, error) {
	return s.find(func(h domain.Webhook) bool { return h.SecretHash == hash })
}

func (s *Webhooks) In(_ context.Context, conv domain.ConversationID) ([]*domain.Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Webhook{}
	for _, h := range s.all {
		if h.Conversation == conv {
			c := h
			out = append(out, &c)
		}
	}
	return out, nil
}
