// Package application holds the Call context use cases (ADR-0006): joining, leaving and media
// state of huddles. Only members of the conversation take part; a call of a conversation the
// actor may not read looks missing.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrNoConversation = errors.New("conversation not found")
	ErrNotMember      = errors.New("join the conversation first")
)

// Conversations is the port to the Chat context: the members of a conversation actor belongs to
// (ErrNoConversation when actor may not read it, ErrNotMember when actor may read but did not join).
type Conversations interface {
	Members(ctx context.Context, actor, conversation string) ([]string, error)
}

// Notifier tells the conversation's members' open tabs about the call's new state.
type Notifier interface {
	Notify(ctx context.Context, to []string, c *domain.Call)
}

type nobody struct{}

func (nobody) Notify(context.Context, []string, *domain.Call) {}

type Deps struct {
	Calls         domain.Repository
	Conversations Conversations
	Notifier      Notifier // optional
	NewID         func() string
	Now           func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Notifier == nil {
		d.Notifier = nobody{}
	}
	return &Service{d: d}
}

// attempts bounds the optimistic-locking retries when people join or leave at the same moment.
const attempts = 5

// Live is the conversation's live call, or nil.
func (s *Service) Live(ctx context.Context, actor, conv string) (*domain.Call, error) {
	if _, err := s.d.Conversations.Members(ctx, actor, conv); err != nil && !errors.Is(err, ErrNotMember) {
		return nil, err
	}
	c, err := s.d.Calls.LiveIn(ctx, domain.ConversationID(conv))
	if errors.Is(err, domain.ErrCallNotFound) {
		return nil, nil
	}
	return c, err
}

// Join starts the conversation's call, or joins the live one.
func (s *Service) Join(ctx context.Context, actor, conv string) (*domain.Call, error) {
	members, err := s.d.Conversations.Members(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	for range attempts {
		c, err := s.d.Calls.LiveIn(ctx, domain.ConversationID(conv))
		switch {
		case errors.Is(err, domain.ErrCallNotFound):
			c = domain.Start(domain.CallID(s.d.NewID()), domain.WorkspaceID(tenant.Workspace(ctx)), domain.ConversationID(conv), domain.UserID(actor), s.d.Now())
			err = s.d.Calls.Create(ctx, c)
			if errors.Is(err, domain.ErrCallLive) {
				continue // someone else started it first: join theirs
			}
		case err == nil:
			if err = c.Join(domain.UserID(actor), s.d.Now()); err != nil {
				return nil, err
			}
			err = s.d.Calls.Save(ctx, c)
		}
		if errors.Is(err, domain.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		s.d.Notifier.Notify(ctx, members, c)
		return c, nil
	}
	return nil, domain.ErrConflict
}

// change applies fn to a call of a conversation actor belongs to, retrying on conflicts.
func (s *Service) change(ctx context.Context, actor, id string, fn func(*domain.Call) error) (*domain.Call, error) {
	for range attempts {
		c, err := s.d.Calls.ByID(ctx, domain.CallID(id))
		if err != nil {
			return nil, err
		}
		if string(c.Workspace) != tenant.Workspace(ctx) {
			return nil, domain.ErrCallNotFound
		}
		members, err := s.d.Conversations.Members(ctx, actor, string(c.Conversation))
		if err != nil {
			return nil, domain.ErrCallNotFound
		}
		if err := fn(c); err != nil {
			return nil, err
		}
		if err := s.d.Calls.Save(ctx, c); errors.Is(err, domain.ErrConflict) {
			continue
		} else if err != nil {
			return nil, err
		}
		s.d.Notifier.Notify(ctx, members, c)
		return c, nil
	}
	return nil, domain.ErrConflict
}

func (s *Service) Leave(ctx context.Context, actor, id string) error {
	_, err := s.change(ctx, actor, id, func(c *domain.Call) error { return c.Leave(domain.UserID(actor), s.d.Now()) })
	return err
}

func (s *Service) SetMedia(ctx context.Context, actor, id string, m domain.Media) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.SetMedia(domain.UserID(actor), m) })
}
