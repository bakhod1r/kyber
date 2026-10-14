// Package application holds the Call context use cases (ADR-0009): joining, leaving and media
// state of huddles. Only members of the conversation take part; a call of a conversation the
// actor may not read looks missing.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrNoConversation = errors.New("conversation not found")
	ErrNotMember      = errors.New("join the conversation first")
	// ErrNoMediaServer: the call is a room but no media server is configured (any more).
	ErrNoMediaServer = errors.New("the media server is not configured")
)

// Rooms is the media server (SFU, ADR-0017). When it is set, new calls are rooms on it; a participant
// enters with a short-lived ticket for that room only.
type Rooms interface {
	Ticket(room, user string, now time.Time) (RoomTicket, error)
	// Evict disconnects user from room (S103); someone already gone is no error.
	Evict(ctx context.Context, room, user string) error
}

// Recorder records room calls on the media server (LiveKit Egress, S104).
type Recorder interface {
	Start(ctx context.Context, room string) (egress string, err error)
	Stop(ctx context.Context, egress string) error // a recording that already ended is no error
	// Open is the finished file; an error while it is still being written (the relay retries).
	Open(ctx context.Context, egress string) (io.ReadCloser, error)
	Discard(ctx context.Context, egress string) error // after the file was posted
}

// ErrRecordingLost: the media server's recording failed or its file is gone; there is nothing to post.
var ErrRecordingLost = errors.New("the media server's recording was lost")

// Recordings posts a finished recording to its conversation, as by did (the Chat context).
type Recordings interface {
	Post(ctx context.Context, by, conversation, name string, body io.Reader) error
}

// RoomTicket is where the browser connects and the signed token that lets it in.
type RoomTicket struct {
	URL, Token string
	Expires    time.Time
}

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
	Rooms         Rooms    // optional: without it every call is a browser mesh
	Recorder      Recorder // optional: without it room calls are recorded in the browser
	Recordings    Recordings
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
			start := domain.Start
			if s.d.Rooms != nil {
				start = domain.StartRoom
			}
			c = start(domain.CallID(s.d.NewID()), domain.WorkspaceID(tenant.Workspace(ctx)), domain.ConversationID(conv), domain.UserID(actor), s.d.Now())
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

// EnterRoom hands a participant of a room call the ticket to its media-server room.
func (s *Service) EnterRoom(ctx context.Context, actor, id string) (RoomTicket, error) {
	c, _, err := s.load(ctx, actor, id)
	if err != nil {
		return RoomTicket{}, err
	}
	if err := c.EnterRoom(domain.UserID(actor)); err != nil {
		return RoomTicket{}, err
	}
	if s.d.Rooms == nil {
		return RoomTicket{}, ErrNoMediaServer
	}
	return s.d.Rooms.Ticket(string(c.ID), actor, s.d.Now())
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

// Remove takes target out of the call; only its host may (S103).
func (s *Service) Remove(ctx context.Context, actor, id, target string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.Remove(domain.UserID(actor), domain.UserID(target), s.d.Now()) })
}

// OnParticipantLeft disconnects whoever left a room call from the media server (call.participant_left).
func (s *Service) OnParticipantLeft(ctx context.Context, m outbox.Message) error {
	var e domain.ParticipantLeft
	if err := json.Unmarshal(m.Payload, &e); err != nil || s.d.Rooms == nil {
		return nil // a poison message must not block the relay
	}
	return s.d.Rooms.Evict(ctx, string(e.Call), string(e.User))
}

// OnRecordingStopped stops the media server's recording and posts the file to the conversation as the one who
// recorded it (call.recording_stopped); the transcript follows as for browser recordings.
func (s *Service) OnRecordingStopped(ctx context.Context, m outbox.Message) error {
	var e domain.RecordingStopped
	if err := json.Unmarshal(m.Payload, &e); err != nil || s.d.Recorder == nil || s.d.Recordings == nil {
		return nil
	}
	if err := s.d.Recorder.Stop(ctx, e.Egress); err != nil {
		return err
	}
	file, err := s.d.Recorder.Open(ctx, e.Egress)
	if errors.Is(err, ErrRecordingLost) {
		return nil
	} else if err != nil {
		return err
	}
	defer file.Close()
	name := "Huddle " + e.Since.UTC().Format("2006-01-02 15-04") + ".mp4"
	if err := s.d.Recordings.Post(ctx, string(e.By), string(e.Conversation), name, file); err != nil {
		return err
	}
	return s.d.Recorder.Discard(ctx, e.Egress)
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

// StartRecording marks the call as recorded by actor (everyone sees it); StopRecording ends that. A room call is
// recorded by the media server when one records (S104), otherwise by actor's browser.
func (s *Service) StartRecording(ctx context.Context, actor, id string) (*domain.Call, error) {
	c, _, err := s.load(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if !c.Room || s.d.Recorder == nil {
		return s.change(ctx, actor, id, func(c *domain.Call) error { return c.StartRecording(domain.UserID(actor), s.d.Now()) })
	}
	if err := c.StartRecording(domain.UserID(actor), s.d.Now()); err != nil {
		return nil, err // checked before the media server starts anything
	}
	egress, err := s.d.Recorder.Start(ctx, string(c.ID))
	if err != nil {
		return nil, err
	}
	c, err = s.change(ctx, actor, id, func(c *domain.Call) error { return c.RecordOnServer(domain.UserID(actor), egress, s.d.Now()) })
	if err != nil {
		_ = s.d.Recorder.Stop(ctx, egress) // someone else started recording meanwhile
	}
	return c, err
}

func (s *Service) StopRecording(ctx context.Context, actor, id string) (*domain.Call, error) {
	return s.change(ctx, actor, id, func(c *domain.Call) error { return c.StopRecording(domain.UserID(actor)) })
}
