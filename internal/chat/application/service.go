// Package app holds the Chat context use cases (ADR-0006): channels, DMs and messages of the
// request's workspace. Conversations a user may not read look missing (404), never forbidden.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var ErrNotWorkspaceMember = errors.New("everyone in a conversation must be a member of the workspace")

const (
	DefaultPage = 50
	MaxPage     = 200
)

// Members is the anti-corruption port to the Workspace context.
type Members interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

type Deps struct {
	Conversations domain.Repository
	Messages      domain.Messages
	Members       Members
	NewID         func() string
	Now           func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// View is a conversation as one user sees it.
type View struct {
	*domain.Conversation
	Unread int
}

func (s *Service) requireMembers(ctx context.Context, ws string, users []string) error {
	for _, u := range users {
		ok, err := s.d.Members.IsMember(ctx, ws, u)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotWorkspaceMember
		}
	}
	return nil
}

// readable loads a conversation of the request's workspace that actor may read.
func (s *Service) readable(ctx context.Context, actor, id string) (*domain.Conversation, error) {
	c, err := s.d.Conversations.ByID(ctx, domain.ConversationID(id))
	if err != nil {
		return nil, err
	}
	if string(c.Workspace) != tenant.Workspace(ctx) || !c.CanRead(domain.UserID(actor)) {
		return nil, domain.ErrConversationNotFound
	}
	return c, nil
}

type NewChannel struct {
	Name, Topic string
	Private     bool
}

func (s *Service) CreateChannel(ctx context.Context, actor string, in NewChannel) (*domain.Conversation, error) {
	c, err := domain.NewChannel(domain.ConversationID(s.d.NewID()), domain.WorkspaceID(tenant.Workspace(ctx)), in.Name, in.Topic,
		in.Private, domain.UserID(actor), s.d.Now())
	if err != nil {
		return nil, err
	}
	return c, s.d.Conversations.Create(ctx, c)
}

// OpenDM returns the DM between actor and others, creating it the first time.
func (s *Service) OpenDM(ctx context.Context, actor string, others []string) (*domain.Conversation, error) {
	ws := tenant.Workspace(ctx)
	ids := make([]domain.UserID, len(others))
	for i, o := range others {
		ids[i] = domain.UserID(o)
	}
	c, err := domain.OpenDM(domain.ConversationID(s.d.NewID()), domain.WorkspaceID(ws), domain.UserID(actor), ids, s.d.Now())
	if err != nil {
		return nil, err
	}
	existing, err := s.d.Conversations.DMByKey(ctx, c.Workspace, c.Key)
	if err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrConversationNotFound) {
		return nil, err
	}
	if err := s.requireMembers(ctx, ws, others); err != nil {
		return nil, err
	}
	return c, s.d.Conversations.Create(ctx, c)
}

// Conversations lists what actor sees in the workspace, with unread counts.
func (s *Service) Conversations(ctx context.Context, actor string) ([]View, error) {
	all, err := s.d.Conversations.Visible(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor))
	if err != nil {
		return nil, err
	}
	unread, err := s.d.Conversations.Unread(ctx, domain.UserID(actor))
	if err != nil {
		return nil, err
	}
	out := make([]View, len(all))
	for i, c := range all {
		out[i] = View{Conversation: c, Unread: unread[c.ID]}
	}
	return out, nil
}

func (s *Service) Conversation(ctx context.Context, actor, id string) (View, error) {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return View{}, err
	}
	unread, err := s.d.Conversations.Unread(ctx, domain.UserID(actor))
	return View{Conversation: c, Unread: unread[c.ID]}, err
}

func (s *Service) Join(ctx context.Context, actor, id string) error {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := c.Join(domain.UserID(actor)); err != nil {
		return err
	}
	return s.d.Conversations.AddMember(ctx, c.ID, domain.UserID(actor), s.d.Now())
}

func (s *Service) Leave(ctx context.Context, actor, id string) error {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := c.Leave(domain.UserID(actor)); err != nil {
		return err
	}
	return s.d.Conversations.RemoveMember(ctx, c.ID, domain.UserID(actor))
}

func (s *Service) Invite(ctx context.Context, actor, id, user string) error {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := c.Invite(domain.UserID(actor), domain.UserID(user)); err != nil {
		return err
	}
	if err := s.requireMembers(ctx, string(c.Workspace), []string{user}); err != nil {
		return err
	}
	return s.d.Conversations.AddMember(ctx, c.ID, domain.UserID(user), s.d.Now())
}

// Post: only members write (join a public channel first).
func (s *Service) Post(ctx context.Context, actor, conv, body string) (*domain.Message, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return nil, domain.ErrNotMember
	}
	now := s.d.Now()
	m, err := domain.NewMessage(domain.MessageID(s.d.NewID()), c.ID, domain.UserID(actor), body, now)
	if err != nil {
		return nil, err
	}
	if err := s.d.Messages.Post(ctx, m); err != nil {
		return nil, err
	}
	// Your own message marks the conversation read up to it.
	return m, s.d.Conversations.MarkRead(ctx, c.ID, domain.UserID(actor), now)
}

// message loads a message in a conversation actor may read.
func (s *Service) message(ctx context.Context, actor, id string) (*domain.Message, error) {
	m, err := s.d.Messages.ByID(ctx, domain.MessageID(id))
	if err != nil {
		return nil, err
	}
	if _, err := s.readable(ctx, actor, string(m.Conversation)); err != nil {
		return nil, domain.ErrMessageNotFound
	}
	return m, nil
}

func (s *Service) Edit(ctx context.Context, actor, id, body string) (*domain.Message, error) {
	m, err := s.message(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if err := m.Edit(domain.UserID(actor), body, s.d.Now()); err != nil {
		return nil, err
	}
	return m, s.d.Messages.Update(ctx, m)
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	m, err := s.message(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := m.Delete(domain.UserID(actor), s.d.Now()); err != nil {
		return err
	}
	return s.d.Messages.Update(ctx, m)
}

// History pages a conversation newest first; before is a message id of the same conversation.
func (s *Service) History(ctx context.Context, actor, conv, before string, limit int) ([]*domain.Message, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	if before != "" {
		if m, err := s.d.Messages.ByID(ctx, domain.MessageID(before)); err != nil || m.Conversation != c.ID {
			return nil, domain.ErrMessageNotFound
		}
	}
	if limit <= 0 {
		limit = DefaultPage
	}
	return s.d.Messages.History(ctx, c.ID, domain.MessageID(before), min(limit, MaxPage))
}

func (s *Service) MarkRead(ctx context.Context, actor, conv string) error {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return domain.ErrNotMember
	}
	return s.d.Conversations.MarkRead(ctx, c.ID, domain.UserID(actor), s.d.Now())
}
