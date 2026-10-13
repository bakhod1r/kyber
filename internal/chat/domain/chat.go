// Package domain is the Chat context (ADR-0005): channels and direct messages of a workspace,
// and the messages posted in them. A public channel is readable by every workspace member;
// private channels and DMs only by their members.
package domain

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidName          = errors.New("a channel name is 1–80 lower-case letters, digits, - or _")
	ErrInvalidBody          = errors.New("a message is 1–4000 characters")
	ErrDMSize               = errors.New("a direct message has 2 to 8 people")
	ErrNotJoinable          = errors.New("this conversation cannot be joined or left this way")
	ErrNotMember            = errors.New("not a member of this conversation")
	ErrNotAuthor            = errors.New("only the author can change a message")
	ErrDeleted              = errors.New("the message was deleted")
	ErrNameTaken            = errors.New("a channel with this name already exists")
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
)

const (
	MaxBody      = 4000
	MaxDMMembers = 8
)

type (
	ConversationID string
	MessageID      string
	UserID         string
	WorkspaceID    string
)

type Kind string

const (
	KindChannel Kind = "channel"
	KindDM      Kind = "dm"
)

var channelName = regexp.MustCompile(`^[a-z0-9_-]{1,80}$`)

// Conversation is a channel or a direct message and its members (sorted, unique).
type Conversation struct {
	ID        ConversationID
	Workspace WorkspaceID
	Kind      Kind
	Name      string // channels only
	Topic     string
	Private   bool   // always true for DMs
	Key       string // DMs only: the sorted member ids; one DM per set of people
	CreatedBy UserID
	CreatedAt time.Time
	Members   []UserID
}

func NewChannel(id ConversationID, ws WorkspaceID, name, topic string, private bool, creator UserID, now time.Time) (*Conversation, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !channelName.MatchString(name) {
		return nil, ErrInvalidName
	}
	return &Conversation{ID: id, Workspace: ws, Kind: KindChannel, Name: name, Topic: strings.TrimSpace(topic), Private: private,
		CreatedBy: creator, CreatedAt: now.UTC(), Members: []UserID{creator}}, nil
}

// OpenDM is a direct message between the opener and others (2–8 people in all).
func OpenDM(id ConversationID, ws WorkspaceID, opener UserID, others []UserID, now time.Time) (*Conversation, error) {
	members := append(slices.Clone(others), opener)
	slices.Sort(members)
	members = slices.Compact(members)
	if len(members) < 2 || len(members) > MaxDMMembers {
		return nil, ErrDMSize
	}
	return &Conversation{ID: id, Workspace: ws, Kind: KindDM, Private: true, Key: DMKey(members), CreatedBy: opener,
		CreatedAt: now.UTC(), Members: members}, nil
}

// DMKey identifies a DM by its (sorted, unique) members.
func DMKey(members []UserID) string {
	s := make([]string, len(members))
	for i, m := range members {
		s[i] = string(m)
	}
	return strings.Join(s, ",")
}

func (c *Conversation) IsMember(u UserID) bool {
	_, ok := slices.BinarySearch(c.Members, u)
	return ok
}

// CanRead: everyone in the workspace reads a public channel; others only their own.
func (c *Conversation) CanRead(u UserID) bool { return !c.Private || c.IsMember(u) }

func (c *Conversation) add(u UserID) {
	if i, ok := slices.BinarySearch(c.Members, u); !ok {
		c.Members = slices.Insert(c.Members, i, u)
	}
}

// Join adds u to a public channel (no-op when already in).
func (c *Conversation) Join(u UserID) error {
	if c.Kind != KindChannel || c.Private {
		return ErrNotJoinable
	}
	c.add(u)
	return nil
}

// Invite: a member adds someone to a channel (the only way into a private one).
func (c *Conversation) Invite(by, u UserID) error {
	if c.Kind != KindChannel {
		return ErrNotJoinable
	}
	if !c.IsMember(by) {
		return ErrNotMember
	}
	c.add(u)
	return nil
}

func (c *Conversation) Leave(u UserID) error {
	if c.Kind != KindChannel {
		return ErrNotJoinable
	}
	i, ok := slices.BinarySearch(c.Members, u)
	if !ok {
		return ErrNotMember
	}
	c.Members = slices.Delete(c.Members, i, i+1)
	return nil
}

// Message is one post in a conversation. A deleted message keeps its place in history.
type Message struct {
	ID           MessageID
	Conversation ConversationID
	Author       UserID
	Body         string
	CreatedAt    time.Time
	EditedAt     time.Time // zero unless edited
	Deleted      bool
}

func body(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > MaxBody {
		return "", ErrInvalidBody
	}
	return s, nil
}

func NewMessage(id MessageID, conv ConversationID, author UserID, text string, now time.Time) (*Message, error) {
	b, err := body(text)
	if err != nil {
		return nil, err
	}
	return &Message{ID: id, Conversation: conv, Author: author, Body: b, CreatedAt: now.UTC()}, nil
}

func (m *Message) change(actor UserID) error {
	if m.Deleted {
		return ErrDeleted
	}
	if actor != m.Author {
		return ErrNotAuthor
	}
	return nil
}

func (m *Message) Edit(actor UserID, text string, now time.Time) error {
	if err := m.change(actor); err != nil {
		return err
	}
	b, err := body(text)
	if err != nil {
		return err
	}
	m.Body, m.EditedAt = b, now.UTC()
	return nil
}

func (m *Message) Delete(actor UserID, now time.Time) error {
	if err := m.change(actor); err != nil {
		return err
	}
	m.Body, m.Deleted, m.EditedAt = "", true, now.UTC()
	return nil
}

// Repository stores conversations and their members' read cursors.
type Repository interface {
	Create(ctx context.Context, c *Conversation) error // ErrNameTaken for a channel name used in the workspace
	ByID(ctx context.Context, id ConversationID) (*Conversation, error)
	DMByKey(ctx context.Context, ws WorkspaceID, key string) (*Conversation, error) // ErrConversationNotFound
	AddMember(ctx context.Context, id ConversationID, u UserID, at time.Time) error // no-op when already in
	RemoveMember(ctx context.Context, id ConversationID, u UserID) error
	// Visible lists the workspace's public channels and every conversation u belongs to, by name then creation.
	Visible(ctx context.Context, ws WorkspaceID, u UserID) ([]*Conversation, error)
	MarkRead(ctx context.Context, id ConversationID, u UserID, at time.Time) error
	// Unread counts, per conversation u belongs to, live messages by others newer than u's read cursor.
	Unread(ctx context.Context, u UserID) (map[ConversationID]int, error)
}

// Messages stores messages.
type Messages interface {
	Post(ctx context.Context, m *Message) error
	Update(ctx context.Context, m *Message) error // body, edited, deleted
	ByID(ctx context.Context, id MessageID) (*Message, error)
	// History lists up to limit messages of a conversation older than before (all when empty), newest first.
	History(ctx context.Context, conv ConversationID, before MessageID, limit int) ([]*Message, error)
}
