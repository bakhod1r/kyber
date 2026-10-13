// Package domain is the Chat context (ADR-0009): channels and direct messages of a workspace,
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
	"unicode"
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
	ErrNestedThread         = errors.New("a reply cannot have replies")
	ErrInvalidEmoji         = errors.New("a reaction is one emoji or :name: of up to 32 characters")
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
	Thread       MessageID   // the parent message; empty for a top-level message
	ReplyCount   int         // read model: live replies of a top-level message
	Reactions    []Reaction  // sorted by emoji
	Kind         MessageKind // "" for people's messages
	Ref          string      // system messages: what they are about (unique), e.g. "call:<id>:started"
	File         *File       // an attached file, if any
	PinnedBy     UserID      // empty unless pinned
	PinnedAt     time.Time
	events       []MessageWritten
}

// MessageKind marks messages the system writes into a conversation.
type MessageKind string

const (
	KindHuddleStarted MessageKind = "huddle_started"
	KindHuddleEnded   MessageKind = "huddle_ended"
)

// ErrDuplicateRef: a system message about this already exists (events may be delivered twice).
var ErrDuplicateRef = errors.New("a message about this already exists")

// NewSystemMessage is written by Kyber on someone's behalf (e.g. "started a huddle"); nobody can edit it.
func NewSystemMessage(id MessageID, conv ConversationID, about UserID, kind MessageKind, ref, text string, now time.Time) *Message {
	return &Message{ID: id, Conversation: conv, Author: about, Body: text, CreatedAt: now.UTC(), Kind: kind, Ref: ref}
}

// PullEvents returns and clears the recorded published events.
func (m *Message) PullEvents() []MessageWritten {
	ev := m.events
	m.events = nil
	return ev
}

func (m *Message) written() {
	m.events = append(m.events, MessageWritten{ID: m.ID, Conversation: m.Conversation, Thread: m.Thread, Author: m.Author, Body: m.Body})
}

// Reaction is one emoji and who reacted with it (sorted).
type Reaction struct {
	Emoji string
	Users []UserID
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
	m := &Message{ID: id, Conversation: conv, Author: author, Body: b, CreatedAt: now.UTC()}
	m.written()
	return m, nil
}

// MaxFile bounds one attachment (bytes).
const MaxFile = 25 << 20

var ErrInvalidFile = errors.New("a file has a name and 1 byte to 25 MB")

// File is an attachment; its bytes live in the blob store under the message's id.
type File struct {
	Name string
	Type string // detected by the server, never taken from the client
	Size int64
}

// NewFileMessage posts a file; the message body is its name (searchable).
func NewFileMessage(id MessageID, conv ConversationID, author UserID, f File, now time.Time) (*Message, error) {
	f.Name = strings.TrimSpace(f.Name)
	if i := strings.LastIndexAny(f.Name, `/\`); i >= 0 {
		f.Name = f.Name[i+1:]
	}
	if f.Name == "" || utf8.RuneCountInString(f.Name) > 255 || f.Size <= 0 || f.Size > MaxFile {
		return nil, ErrInvalidFile
	}
	m, err := NewMessage(id, conv, author, f.Name, now)
	if err != nil {
		return nil, err
	}
	m.File = &f
	return m, nil
}

// Pin keeps the message at hand for the conversation (no-op when already pinned).
func (m *Message) Pin(by UserID, now time.Time) error {
	if m.Deleted {
		return ErrDeleted
	}
	if m.PinnedBy == "" {
		m.PinnedBy, m.PinnedAt = by, now.UTC()
	}
	return nil
}

func (m *Message) Unpin() { m.PinnedBy, m.PinnedAt = "", time.Time{} }

// NewReply answers a top-level message in its thread.
func NewReply(id MessageID, parent *Message, author UserID, text string, now time.Time) (*Message, error) {
	if parent.Thread != "" {
		return nil, ErrNestedThread
	}
	if parent.Deleted {
		return nil, ErrDeleted
	}
	b, err := body(text)
	if err != nil {
		return nil, err
	}
	m := &Message{ID: id, Conversation: parent.Conversation, Thread: parent.ID, Author: author, Body: b, CreatedAt: now.UTC()}
	m.written()
	return m, nil
}

// React toggles u's emoji on the message and reports whether it is now on.
func (m *Message) React(u UserID, emoji string) (bool, error) {
	if m.Deleted {
		return false, ErrDeleted
	}
	if emoji == "" || strings.ContainsFunc(emoji, unicode.IsSpace) || utf8.RuneCountInString(emoji) > 32 {
		return false, ErrInvalidEmoji
	}
	i, found := slices.BinarySearchFunc(m.Reactions, emoji, func(r Reaction, e string) int { return strings.Compare(r.Emoji, e) })
	if !found {
		m.Reactions = slices.Insert(m.Reactions, i, Reaction{Emoji: emoji, Users: []UserID{u}})
		return true, nil
	}
	r := &m.Reactions[i]
	if j, in := slices.BinarySearch(r.Users, u); !in {
		r.Users = slices.Insert(r.Users, j, u)
		return true, nil
	} else {
		r.Users = slices.Delete(r.Users, j, j+1)
	}
	if len(r.Users) == 0 {
		m.Reactions = slices.Delete(m.Reactions, i, i+1)
	}
	return false, nil
}

func (m *Message) change(actor UserID) error {
	if m.Deleted {
		return ErrDeleted
	}
	if actor != m.Author || m.Kind != "" {
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
	m.Body, m.Deleted, m.EditedAt, m.Reactions, m.File = "", true, now.UTC(), nil, nil
	m.Unpin()
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
	// Unread counts, per conversation u belongs to, live top-level messages by others newer than u's read cursor.
	Unread(ctx context.Context, u UserID) (map[ConversationID]int, error)
}

// Messages stores messages.
type Messages interface {
	Post(ctx context.Context, m *Message) error   // ErrDuplicateRef for a second system message with the same Ref
	Update(ctx context.Context, m *Message) error // body, edited, deleted, reactions
	ByID(ctx context.Context, id MessageID) (*Message, error)
	// History lists up to limit top-level messages of a conversation older than before (all when empty),
	// newest first, with their reply counts.
	History(ctx context.Context, conv ConversationID, before MessageID, limit int) ([]*Message, error)
	// Replies lists a thread, oldest first.
	Replies(ctx context.Context, parent MessageID) ([]*Message, error)
	// Pinned lists a conversation's pinned messages, most recently pinned first.
	Pinned(ctx context.Context, conv ConversationID) ([]*Message, error)
	// Search finds live messages whose body contains every word of query (case-insensitive) in the given
	// conversations, newest first.
	Search(ctx context.Context, convs []ConversationID, query string, limit int) ([]*Message, error)
}

// MaxPreviews caps the issues previewed for one message.
const MaxPreviews = 5

// issueKey: a project key (2–10 upper-case letters/digits, starting with a letter) and a number from 1.
var issueKey = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])([A-Z][A-Z0-9]{1,9}-[1-9][0-9]{0,8})\b`)

// IssueKeys lists the distinct issue keys written in a message body, in order.
func IssueKeys(body string) []string {
	var out []string
	for _, m := range issueKey.FindAllStringSubmatch(body, -1) {
		if !slices.Contains(out, m[1]) && len(out) < MaxPreviews {
			out = append(out, m[1])
		}
	}
	return out
}
