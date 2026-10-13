// Package memory is the in-memory Chat store (tests, dev mode).
package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

type member struct {
	conv domain.ConversationID
	user domain.UserID
}

type Repository struct {
	mu       sync.Mutex
	convs    map[domain.ConversationID]domain.Conversation
	readAt   map[member]time.Time
	messages map[domain.MessageID]domain.Message
}

func NewRepository() *Repository {
	return &Repository{convs: map[domain.ConversationID]domain.Conversation{}, readAt: map[member]time.Time{},
		messages: map[domain.MessageID]domain.Message{}}
}

func clone(c domain.Conversation) *domain.Conversation {
	c.Members = slices.Clone(c.Members)
	return &c
}

func (r *Repository) Create(_ context.Context, c *domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.Kind == domain.KindChannel {
		for _, o := range r.convs {
			if o.Workspace == c.Workspace && o.Kind == domain.KindChannel && o.Name == c.Name {
				return domain.ErrNameTaken
			}
		}
	}
	r.convs[c.ID] = *clone(*c)
	return nil
}

func (r *Repository) ByID(_ context.Context, id domain.ConversationID) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return nil, domain.ErrConversationNotFound
	}
	return clone(c), nil
}

func (r *Repository) DMByKey(_ context.Context, ws domain.WorkspaceID, key string) (*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.convs {
		if c.Workspace == ws && c.Kind == domain.KindDM && c.Key == key {
			return clone(c), nil
		}
	}
	return nil, domain.ErrConversationNotFound
}

func (r *Repository) AddMember(_ context.Context, id domain.ConversationID, u domain.UserID, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return domain.ErrConversationNotFound
	}
	if i, in := slices.BinarySearch(c.Members, u); !in {
		c.Members = slices.Insert(slices.Clone(c.Members), i, u)
		r.convs[id] = c
	}
	return nil
}

func (r *Repository) RemoveMember(_ context.Context, id domain.ConversationID, u domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return domain.ErrConversationNotFound
	}
	c.Members = slices.DeleteFunc(slices.Clone(c.Members), func(m domain.UserID) bool { return m == u })
	r.convs[id] = c
	delete(r.readAt, member{id, u})
	return nil
}

func (r *Repository) Visible(_ context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Conversation
	for _, c := range r.convs {
		if c.Workspace == ws && c.CanRead(u) {
			out = append(out, clone(c))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Conversation) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r *Repository) MarkRead(_ context.Context, id domain.ConversationID, u domain.UserID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.convs[id]; ok && c.IsMember(u) {
		r.readAt[member{id, u}] = at
	}
	return nil
}

func (r *Repository) Unread(_ context.Context, u domain.UserID) (map[domain.ConversationID]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[domain.ConversationID]int{}
	for _, m := range r.messages {
		c := r.convs[m.Conversation]
		if c.IsMember(u) && m.Thread == "" && m.Author != u && !m.Deleted && m.CreatedAt.After(r.readAt[member{c.ID, u}]) {
			out[c.ID]++
		}
	}
	return out, nil
}

// Messages is the message store over the same data.
func (r *Repository) Messages() *Messages { return &Messages{r} }

type Messages struct{ r *Repository }

func cloneMsg(m domain.Message) domain.Message {
	m.Reactions = slices.Clone(m.Reactions)
	for i := range m.Reactions {
		m.Reactions[i].Users = slices.Clone(m.Reactions[i].Users)
	}
	return m
}

// read returns a copy with its reply count (caller holds the lock).
func (s *Messages) read(m domain.Message) *domain.Message {
	c := cloneMsg(m)
	c.ReplyCount = 0
	for _, o := range s.r.messages {
		if o.Thread == m.ID && m.Thread == "" && !o.Deleted {
			c.ReplyCount++
		}
	}
	return &c
}

func (s *Messages) Post(_ context.Context, m *domain.Message) error {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	if _, ok := s.r.convs[m.Conversation]; !ok {
		return domain.ErrConversationNotFound
	}
	s.r.messages[m.ID] = cloneMsg(*m)
	return nil
}

func (s *Messages) Update(_ context.Context, m *domain.Message) error {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	if _, ok := s.r.messages[m.ID]; !ok {
		return domain.ErrMessageNotFound
	}
	s.r.messages[m.ID] = cloneMsg(*m)
	return nil
}

func (s *Messages) ByID(_ context.Context, id domain.MessageID) (*domain.Message, error) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	m, ok := s.r.messages[id]
	if !ok {
		return nil, domain.ErrMessageNotFound
	}
	return s.read(m), nil
}

func newer(a, b domain.Message) int {
	return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
}

func (s *Messages) History(_ context.Context, conv domain.ConversationID, before domain.MessageID, limit int) ([]*domain.Message, error) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	cursor, hasCursor := s.r.messages[before]
	var all []domain.Message
	for _, m := range s.r.messages {
		if m.Conversation == conv && m.Thread == "" && (!hasCursor || newer(cursor, m) < 0) {
			all = append(all, m)
		}
	}
	slices.SortFunc(all, newer)
	out := make([]*domain.Message, 0, min(limit, len(all)))
	for i := 0; i < len(all) && i < limit; i++ {
		out = append(out, s.read(all[i]))
	}
	return out, nil
}

func (s *Messages) Replies(_ context.Context, parent domain.MessageID) ([]*domain.Message, error) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	var out []*domain.Message
	for _, m := range s.r.messages {
		if m.Thread == parent {
			out = append(out, s.read(m))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Message) int { return newer(*b, *a) })
	return out, nil
}
