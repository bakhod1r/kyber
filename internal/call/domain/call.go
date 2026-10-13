// Package domain is the Call context (ADR-0009): huddles — live audio/video calls with screen
// sharing in a chat conversation. Media flows browser to browser (WebRTC mesh); the server
// keeps who is in the call and relays signaling. At most MaxParticipants people per call, one
// live call per conversation, and the call ends when the last participant leaves.
package domain

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrCallFull     = errors.New("the call is full")
	ErrCallEnded    = errors.New("the call has ended")
	ErrNotInCall    = errors.New("not in this call")
	ErrCallNotFound = errors.New("call not found")
	ErrCallLive     = errors.New("this conversation already has a live call")
	ErrConflict     = errors.New("the call changed meanwhile")
)

// MaxParticipants keeps a full-mesh call within every participant's upload bandwidth.
const MaxParticipants = 6

type (
	CallID         string
	UserID         string
	WorkspaceID    string
	ConversationID string
)

// Media is what a participant sends.
type Media struct{ Audio, Video, Screen bool }

type Participant struct {
	User     UserID
	JoinedAt time.Time
	Media    Media
}

type Call struct {
	ID           CallID
	Workspace    WorkspaceID
	Conversation ConversationID
	StartedBy    UserID
	StartedAt    time.Time
	EndedAt      time.Time     // zero while live
	Participants []Participant // in joining order
	Presentation *Presentation // what everyone looks at together, if anything
	Version      int           // optimistic locking
	events       []Event
}

// Event is a published fact about a call (written to the outbox with it); Chat shows huddles in the
// conversation from them.
type Event interface{ EventName() string }

type CallStarted struct {
	Call         CallID         `json:"call_id"`
	Conversation ConversationID `json:"conversation_id"`
	By           UserID         `json:"by"`
}

type CallEnded struct {
	Call         CallID         `json:"call_id"`
	Conversation ConversationID `json:"conversation_id"`
	By           UserID         `json:"by"` // who started it
	Seconds      int            `json:"seconds"`
}

func (CallStarted) EventName() string { return "call.started" }
func (CallEnded) EventName() string   { return "call.ended" }

// PullEvents returns and clears the recorded events.
func (c *Call) PullEvents() []Event {
	ev := c.events
	c.events = nil
	return ev
}

// A Presentation is a shared file opened for everyone in the call ("present to all"); each viewer opens it
// with their own permissions.
type PresentKind string

const (
	PresentDoc        PresentKind = "doc"
	PresentWhiteboard PresentKind = "whiteboard"
)

type Presentation struct {
	Kind PresentKind
	Ref  string // the doc or whiteboard id
	By   UserID
}

var (
	ErrInvalidPresentation = errors.New("present a doc or a whiteboard")
	ErrNotPresenter        = errors.New("only the presenter can do that")
)

// Present opens a doc or whiteboard for everyone, replacing any current presentation.
func (c *Call) Present(by UserID, kind PresentKind, ref string) error {
	if !c.Live() || !c.In(by) {
		return ErrNotInCall
	}
	if (kind != PresentDoc && kind != PresentWhiteboard) || ref == "" || len(ref) > 200 {
		return ErrInvalidPresentation
	}
	c.Presentation = &Presentation{Kind: kind, Ref: ref, By: by}
	return nil
}

func (c *Call) StopPresenting(by UserID) error {
	if c.Presentation == nil || c.Presentation.By != by {
		return ErrNotPresenter
	}
	c.Presentation = nil
	return nil
}

// CanFollow: only the presenter steers where everyone looks (follow me).
func (c *Call) CanFollow(by UserID) error {
	if !c.Live() || c.Presentation == nil || c.Presentation.By != by {
		return ErrNotPresenter
	}
	return nil
}

// Start opens a call; the starter joins with the microphone on.
func Start(id CallID, ws WorkspaceID, conv ConversationID, by UserID, now time.Time) *Call {
	now = now.UTC()
	return &Call{ID: id, Workspace: ws, Conversation: conv, StartedBy: by, StartedAt: now,
		Participants: []Participant{{User: by, JoinedAt: now, Media: Media{Audio: true}}},
		events:       []Event{CallStarted{Call: id, Conversation: conv, By: by}}}
}

func (c *Call) Live() bool { return c.EndedAt.IsZero() }

func (c *Call) index(u UserID) int {
	return slices.IndexFunc(c.Participants, func(p Participant) bool { return p.User == u })
}

func (c *Call) In(u UserID) bool { return c.index(u) >= 0 }

// Join adds u with the microphone on (no-op when already in).
func (c *Call) Join(u UserID, now time.Time) error {
	switch {
	case !c.Live():
		return ErrCallEnded
	case c.In(u):
		return nil
	case len(c.Participants) >= MaxParticipants:
		return ErrCallFull
	}
	c.Participants = append(c.Participants, Participant{User: u, JoinedAt: now.UTC(), Media: Media{Audio: true}})
	return nil
}

// Leave removes u; the call ends with its last participant.
func (c *Call) Leave(u UserID, now time.Time) error {
	i := c.index(u)
	if !c.Live() || i < 0 {
		return ErrNotInCall
	}
	c.Participants = slices.Delete(c.Participants, i, i+1)
	if c.Presentation != nil && c.Presentation.By == u {
		c.Presentation = nil
	}
	if len(c.Participants) == 0 {
		c.EndedAt = now.UTC()
		c.events = append(c.events, CallEnded{Call: c.ID, Conversation: c.Conversation, By: c.StartedBy, Seconds: int(c.EndedAt.Sub(c.StartedAt).Seconds())})
	}
	return nil
}

// SetMedia sets what u sends; starting a screen share stops anyone else's.
func (c *Call) SetMedia(u UserID, m Media) error {
	i := c.index(u)
	if !c.Live() || i < 0 {
		return ErrNotInCall
	}
	if m.Screen {
		for j := range c.Participants {
			c.Participants[j].Media.Screen = false
		}
	}
	c.Participants[i].Media = m
	return nil
}

// Repository stores calls.
type Repository interface {
	Create(ctx context.Context, c *Call) error // ErrCallLive when the conversation already has one
	// Save writes the call if its Version is still the stored one (then increments it), else ErrConflict.
	Save(ctx context.Context, c *Call) error
	ByID(ctx context.Context, id CallID) (*Call, error)
	LiveIn(ctx context.Context, conv ConversationID) (*Call, error) // ErrCallNotFound when none
	CountLive(ctx context.Context) (int, error)                     // live calls in all workspaces (a metric)
}

// WebRTC signaling between two participants; relayed, never stored.
type SignalKind string

const (
	SignalOffer     SignalKind = "offer"
	SignalAnswer    SignalKind = "answer"
	SignalCandidate SignalKind = "candidate"
	// MaxSignal bounds one SDP or ICE candidate (bytes).
	MaxSignal = 64 << 10
)

var ErrInvalidSignal = errors.New("a signal is an offer, answer or candidate of at most 64 KB to another participant")

type Signal struct {
	Call     CallID
	From, To UserID
	Kind     SignalKind
	Data     string // opaque to the server (SDP or ICE candidate JSON)
}

// Signal checks that from may send kind/data to to in this call.
func (c *Call) Signal(from, to UserID, kind, data string) (Signal, error) {
	if !c.Live() || !c.In(from) || !c.In(to) {
		return Signal{}, ErrNotInCall
	}
	k := SignalKind(kind)
	if from == to || (k != SignalOffer && k != SignalAnswer && k != SignalCandidate) || data == "" || len(data) > MaxSignal {
		return Signal{}, ErrInvalidSignal
	}
	return Signal{Call: c.ID, From: from, To: to, Kind: k, Data: data}, nil
}
