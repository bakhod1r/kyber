package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

// Bookmarks stores bookmarks in memory.
type Bookmarks struct {
	mu  sync.Mutex
	all []domain.Bookmark // in the order added
}

func NewBookmarks() *Bookmarks { return &Bookmarks{} }

func (s *Bookmarks) Add(_ context.Context, b *domain.Bookmark) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = append(s.all, *b)
	return nil
}

func (s *Bookmarks) Remove(_ context.Context, id domain.BookmarkID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all = slices.DeleteFunc(s.all, func(b domain.Bookmark) bool { return b.ID == id })
	return nil
}

func (s *Bookmarks) ByID(_ context.Context, id domain.BookmarkID) (*domain.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.all {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, domain.ErrBookmarkNotFound
}

func (s *Bookmarks) In(_ context.Context, conv domain.ConversationID) ([]*domain.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.Bookmark{}
	for _, b := range s.all {
		if b.Conversation == conv {
			c := b
			out = append(out, &c)
		}
	}
	return out, nil
}
