// Package application holds the Call context use cases (ADR-0009): joining, leaving and media
// state of huddles. Only members of the conversation take part; a call of a conversation the
// actor may not read looks missing.
package application

import (
	"context"
	"errors"
	"strings"
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

// Notifier tells the conversation's members' open tabs about the call's new state, and the participants
// where the presenter is looking (follow me; ephemeral).
type Notifier interface {
	Notify(ctx context.Context, to []string, c *domain.Call)
	Follow(ctx context.Context, to []string, c *domain.Call, view View)
	// React shows a participant's emoji to the others (S75, not stored).
	React(ctx context.Context, to []string, c *domain.Call, from, emoji string)
}

// View is the presenter's position in the presented file (fractions of its size, and zoom).
type View struct{ X, Y, Zoom float64 }

// Signals delivers WebRTC signaling to its one recipient (never stored).
type Signals interface {
	Relay(ctx context.Context, s domain.Signal)
}

type nobody struct{}

func (nobody) Notify(context.Context, []string, *domain.Call)                {}
func (nobody) Follow(context.Context, []string, *domain.Call, View)          {}
func (nobody) React(context.Context, []string, *domain.Call, string, string) {}
func (nobody) Relay(context.Context, domain.Signal)                          {}

type Deps struct {
	Calls         domain.Repository
	Conversations Conversations
	Notifier      Notifier // optional
	Signals       Signals  // optional
	NewID         func() string
	Now           func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Notifier == nil {
		d.Notifier = nobody{}
	}
	if d.Signals == nil {
		d.Signals = nobody{}
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

// load is a call of the request's workspace in a conversation actor belongs to.
func (s *Service) load(ctx context.Context, actor, id string) (*domain.Call, []string, error) {
	c, err := s.d.Calls.ByID(ctx, domain.CallID(id))
	if err != nil {
		return nil, nil, err
	}
	if string(c.Workspace) != tenant.Workspace(ctx) {
		return nil, nil, domain.ErrCallNotFound
	}
	members, err := s.d.Conversations.Members(ctx, actor, string(c.Conversation))
	if err != nil {
		return nil, nil, domain.ErrCallNotFound
	}
	return c, members, nil
}

// Signal relays an offer, answer or ICE candidate from actor to another participant.
func (s *Service) Signal(ctx context.Context, actor, id, to, kind, data string) error {
	c, _, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	sig, err := c.Signal(domain.UserID(actor), domain.UserID(to), kind, data)
	if err != nil {
		return err
	}
	s.d.Signals.Relay(ctx, sig)
	return nil
}

// change applies fn to a call of a conversation actor belongs to, retrying on conflicts.
func (s *Service) change(ctx context.Context, actor, id string, fn func(*domain.Call) error) (*domain.Call, error) {
	for range attempts {
		c, members, err := s.load(ctx, actor, id)
		if err != nil {
			return nil, err
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

// Present opens a doc or whiteboard for everyone in the call.
func (s *Service) Present(ctx context.Context, actor, id string, kind domain.PresentKind, ref string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.Present(domain.UserID(actor), kind, ref) })
}

func (s *Service) StopPresenting(ctx context.Context, actor, id string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.StopPresenting(domain.UserID(actor)) })
}

// Follow tells the other participants where the presenter is looking.
func (s *Service) Follow(ctx context.Context, actor, id string, v View) error {
	c, _, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := c.CanFollow(domain.UserID(actor)); err != nil {
		return err
	}
	var to []string
	for _, p := range c.Participants {
		if string(p.User) != actor {
			to = append(to, string(p.User))
		}
	}
	s.d.Notifier.Follow(ctx, to, c, v)
	return nil
}

// RaiseHand raises or lowers the actor's hand in the call (S75).
func (s *Service) RaiseHand(ctx context.Context, actor, id string, raised bool) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.RaiseHand(domain.UserID(actor), raised) })
}

// React shows an emoji from the actor to the other participants (not stored).
func (s *Service) React(ctx context.Context, actor, id, emoji string) error {
	c, _, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := c.CanReact(domain.UserID(actor), emoji); err != nil {
		return err
	}
	var to []string
	for _, p := range c.Participants {
		if string(p.User) != actor {
			to = append(to, string(p.User))
		}
	}
	s.d.Notifier.React(ctx, to, c, actor, strings.TrimSpace(emoji))
	return nil
}

// LiveCalls counts live calls across workspaces (operations metric).
func (s *Service) LiveCalls(ctx context.Context) (int, error) { return s.d.Calls.CountLive(ctx) }

// StartRecording marks the call as recorded by actor (everyone sees it); StopRecording ends that.
func (s *Service) StartRecording(ctx context.Context, actor, id string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.StartRecording(domain.UserID(actor), s.d.Now()) })
}

func (s *Service) StopRecording(ctx context.Context, actor, id string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.StopRecording(domain.UserID(actor)) })
}
