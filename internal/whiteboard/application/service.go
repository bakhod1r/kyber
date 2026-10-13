// Package application holds the Whiteboard use cases: boards of a conversation, drawn by its members.
package application

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

var ErrNoConversation = errors.New("conversation not found")

// MaxOps bounds one request.
const MaxOps = 200

var ErrTooManyOps = errors.New("at most 200 changes at once")

// Conversations is the port to the Chat context: the members of a conversation actor belongs to.
type Conversations interface {
	Members(ctx context.Context, actor, conversation string) ([]string, error)
}

// Notifier tells the members' open tabs about applied changes and live cursors.
type Notifier interface {
	Ops(ctx context.Context, to []string, board domain.WhiteboardID, by string, applied []domain.OpResult)
	Cursor(ctx context.Context, to []string, board domain.WhiteboardID, by string, x, y float64)
}

type nobody struct{}

func (nobody) Ops(context.Context, []string, domain.WhiteboardID, string, []domain.OpResult)   {}
func (nobody) Cursor(context.Context, []string, domain.WhiteboardID, string, float64, float64) {}

type Deps struct {
	Boards        domain.Repository
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

func (s *Service) Create(ctx context.Context, actor, conv, title string) (*domain.Whiteboard, error) {
	if _, err := s.d.Conversations.Members(ctx, actor, conv); err != nil {
		return nil, err
	}
	b, err := domain.New(domain.WhiteboardID(s.d.NewID()), domain.WorkspaceID(tenant.Workspace(ctx)), domain.ConversationID(conv), title,
		domain.UserID(actor), s.d.Now())
	if err != nil {
		return nil, err
	}
	return b, s.d.Boards.Create(ctx, b)
}

func (s *Service) List(ctx context.Context, actor, conv string) ([]*domain.Whiteboard, error) {
	if _, err := s.d.Conversations.Members(ctx, actor, conv); err != nil {
		return nil, err
	}
	return s.d.Boards.InConversation(ctx, domain.ConversationID(conv))
}

// load is a board of the request's workspace whose conversation actor belongs to (else it looks missing).
func (s *Service) load(ctx context.Context, actor, id string) (*domain.Whiteboard, []string, error) {
	b, err := s.d.Boards.ByID(ctx, domain.WhiteboardID(id))
	if err != nil {
		return nil, nil, err
	}
	if string(b.Workspace) != tenant.Workspace(ctx) {
		return nil, nil, domain.ErrWhiteboardMissing
	}
	members, err := s.d.Conversations.Members(ctx, actor, string(b.Conversation))
	if err != nil {
		return nil, nil, domain.ErrWhiteboardMissing
	}
	return b, members, nil
}

func (s *Service) Get(ctx context.Context, actor, id string) (*domain.Whiteboard, error) {
	b, _, err := s.load(ctx, actor, id)
	return b, err
}

// Apply runs the ops; when another writer saved meanwhile it reloads and applies them again (each op still
// checks its own element version, so nobody's change is silently overwritten).
func (s *Service) Apply(ctx context.Context, actor, id string, ops []domain.Op) ([]domain.OpResult, error) {
	if len(ops) > MaxOps {
		return nil, ErrTooManyOps
	}
	for range 5 {
		b, members, err := s.load(ctx, actor, id)
		if err != nil {
			return nil, err
		}
		res := b.Apply(domain.UserID(actor), ops, s.d.Now())
		applied := slices.DeleteFunc(slices.Clone(res), func(r domain.OpResult) bool { return !r.Applied })
		if len(applied) > 0 {
			if err := s.d.Boards.Save(ctx, b.ID, applied); errors.Is(err, domain.ErrStale) {
				continue
			} else if err != nil {
				return nil, err
			}
			s.d.Notifier.Ops(ctx, members, b.ID, actor, applied)
		}
		return res, nil
	}
	return nil, domain.ErrStale
}

// Cursor shares actor's pointer with the other members (ephemeral).
func (s *Service) Cursor(ctx context.Context, actor, id string, x, y float64) error {
	b, members, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(slices.Clone(members), func(m string) bool { return m == actor })
	s.d.Notifier.Cursor(ctx, others, b.ID, actor, x, y)
	return nil
}
