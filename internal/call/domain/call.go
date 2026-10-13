// Package domain is the Call context (ADR-0006): huddles — live audio/video calls with screen
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
	Version      int           // optimistic locking
}

// Start opens a call; the starter joins with the microphone on.
func Start(id CallID, ws WorkspaceID, conv ConversationID, by UserID, now time.Time) *Call {
	now = now.UTC()
	return &Call{ID: id, Workspace: ws, Conversation: conv, StartedBy: by, StartedAt: now,
		Participants: []Participant{{User: by, JoinedAt: now, Media: Media{Audio: true}}}}
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
	if len(c.Participants) == 0 {
		c.EndedAt = now.UTC()
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
}
