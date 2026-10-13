package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

type prefKey struct {
	user domain.UserID
	conv domain.ConversationID
}

// Prefs stores conversation settings in memory.
type Prefs struct {
	mu  sync.Mutex
	all map[prefKey]domain.Pref
}

func NewPrefs() *Prefs { return &Prefs{all: map[prefKey]domain.Pref{}} }

func (s *Prefs) Put(_ context.Context, p *domain.Pref) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all[prefKey{p.User, p.Conversation}] = *p
	return nil
}

func (s *Prefs) list(keep func(domain.Pref) bool) []*domain.Pref {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Pref{}
	for _, p := range s.all {
		if keep(p) {
			c := p
			out = append(out, &c)
		}
	}
	return out
}

func (s *Prefs) Mine(_ context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Pref, error) {
	return s.list(func(p domain.Pref) bool { return p.Workspace == ws && p.User == u }), nil
}

func (s *Prefs) Of(_ context.Context, c domain.ConversationID) ([]*domain.Pref, error) {
	return s.list(func(p domain.Pref) bool { return p.Conversation == c }), nil
}
