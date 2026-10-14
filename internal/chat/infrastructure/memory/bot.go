package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

// Bots stores the bot list in memory.
type Bots struct {
	mu  sync.Mutex
	all []domain.Bot
}

func NewBots() *Bots { return &Bots{} }

func (s *Bots) Add(_ context.Context, b *domain.Bot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = append(s.all, *b)
	return nil
}

func (s *Bots) In(_ context.Context, ws domain.WorkspaceID) ([]*domain.Bot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Bot{}
	for _, b := range s.all {
		if b.Workspace == ws {
			c := b
			out = append(out, &c)
		}
	}
	return out, nil
}
