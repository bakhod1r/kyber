// Package memory is the in-memory Chat store (tests, dev mode).
package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
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
	outbox   []domain.MessageWritten
	outAt    []outbox.Meta
}

// Outbox returns the published events written so far; OutboxMeta their time and workspace.
func (r *Repository) Outbox() []domain.MessageWritten {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.MessageWritten(nil), r.outbox...)
}

func (r *Repository) OutboxMeta() []outbox.Meta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]outbox.Meta(nil), r.outAt...)
}

func NewRepository() *Repository {
	return &Repository{convs: map[domain.ConversationID]domain.Conversation{}, readAt: map[member]time.Time{},
		messages: map[domain.MessageID]domain.Message{}}
}

func clone(c domain.Conversation) *domain.Conversation {
	c.Members = slices.Clone(c.Members)
	c.Admins = slices.Clone(c.Admins)
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

func (r *Repository) Update(_ context.Context, c *domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.convs[c.ID]
	if !ok {
		return domain.ErrConversationNotFound
	}
	for _, o := range r.convs {
		if o.ID != c.ID && o.Workspace == c.Workspace && o.Kind == domain.KindChannel && o.Name == c.Name {
			return domain.ErrNameTaken
		}
	}
	cur.Name, cur.Topic, cur.Admins, cur.ArchivedAt = c.Name, c.Topic, slices.Clone(c.Admins), c.ArchivedAt
	r.convs[c.ID] = cur
	return nil
}

func (r *Repository) ReadCursors(_ context.Context, id domain.ConversationID) (map[domain.UserID]time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[domain.UserID]time.Time{}
	for k, at := range r.readAt {
		if k.conv == id && !at.IsZero() {
			out[k.user] = at
		}
	}
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
	if m.File != nil {
		f := *m.File
		m.File = &f
	}
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

func (s *Messages) Post(ctx context.Context, m *domain.Message) error {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	if _, ok := s.r.convs[m.Conversation]; !ok {
		return domain.ErrConversationNotFound
	}
	if m.Ref != "" {
		for _, o := range s.r.messages {
			if o.Ref == m.Ref {
				return domain.ErrDuplicateRef
			}
		}
	}
	s.r.messages[m.ID] = cloneMsg(*m)
	at := outbox.MetaOf(ctx, time.Now())
	for _, e := range m.PullEvents() {
		s.r.outbox, s.r.outAt = append(s.r.outbox, e), append(s.r.outAt, at)
	}
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

func (s *Messages) Pinned(_ context.Context, conv domain.ConversationID) ([]*domain.Message, error) {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	var out []*domain.Message
	for _, m := range s.r.messages {
		if m.Conversation == conv && m.PinnedBy != "" {
			out = append(out, s.read(m))
		}
	}
	slices.SortFunc(out, func(a, b *domain.Message) int { return b.PinnedAt.Compare(a.PinnedAt) })
	return out, nil
}

func (s *Messages) Search(_ context.Context, convs []domain.ConversationID, query string, limit int) ([]*domain.Message, error) {
	words := strings.Fields(strings.ToLower(query))
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	var all []domain.Message
	for _, m := range s.r.messages {
		if m.Deleted || len(words) == 0 || !slices.Contains(convs, m.Conversation) {
			continue
		}
		body := strings.ToLower(m.Body)
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(body, w) }) {
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
