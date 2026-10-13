// Package app holds the Chat context use cases (ADR-0009): channels, DMs and messages of the
// request's workspace. Conversations a user may not read look missing (404), never forbidden.
package application

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var ErrQueryTooShort = errors.New("search for at least 2 characters")

// Transcriber turns a recording's audio into text (a speech-to-text service).
type Transcriber interface {
	Transcribe(ctx context.Context, name string, audio io.Reader) (string, error)
}

// Projects tells whether someone may browse a project (Project context).
type Projects interface {
	CanBrowse(ctx context.Context, user, key string) (bool, error)
}

// ErrCannotBrowse: a channel can only be linked to a project its manager may browse.
var ErrCannotBrowse = errors.New("you cannot browse that project")

var ErrNotWorkspaceMember = errors.New("everyone in a conversation must be a member of the workspace")

const (
	DefaultPage = 50
	MaxPage     = 200
)

// Members is the anti-corruption port to the Workspace context.
type Members interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

// Notifier tells members' open tabs about an event (the realtime stream); best effort.
type Notifier interface {
	Notify(ctx context.Context, to []domain.UserID, e domain.Event)
}

type nobody struct{}

func (nobody) Notify(context.Context, []domain.UserID, domain.Event) {}

// IssuePreview is an issue mentioned by key in a message, as the reader may see it.
type IssuePreview struct{ Key, Title, Status string }

// Issues is the port to Issue Tracking: the preview of an issue if user may browse it.
type Issues interface {
	Preview(ctx context.Context, user, key string) (IssuePreview, bool, error)
}

// Blobs stores attachments' bytes (platform/blob) under the message id.
type Blobs interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type Deps struct {
	Blobs         Blobs    // optional: files
	Issues        Issues   // optional: issue previews
	Notifier      Notifier // optional
	Conversations domain.Repository
	Messages      domain.Messages
	Saved         domain.SavedMessages     // optional: save for later (S65)
	Statuses      domain.Statuses          // optional: status and DND (S66)
	Scheduled     domain.ScheduledMessages // optional: scheduled messages (S67)
	Reminders     domain.Reminders         // optional: /remind me (S67)
	Groups        domain.Groups            // optional: user groups (S70)
	Prefs         domain.Prefs             // optional: levels and sections (S71)
	Projects      Projects                 // optional: project channels (S73)
	Transcriber   Transcriber              // optional: recording transcripts (S77); nothing leaves the server without it
	Members       Members
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

// tell notifies the members of the message's conversation.
func (s *Service) tell(ctx context.Context, m *domain.Message, e domain.Event) {
	if c, err := s.d.Conversations.ByID(ctx, m.Conversation); err == nil {
		s.d.Notifier.Notify(ctx, c.Members, e)
	}
}

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
	if err := c.Writable(); err != nil {
		return nil, err
	}
	now := s.d.Now()
	m, err := domain.NewMessage(domain.MessageID(s.d.NewID()), c.ID, domain.UserID(actor), body, now)
	if err != nil {
		return nil, err
	}
	if err := s.d.Messages.Post(ctx, m); err != nil {
		return nil, err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *m})
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
	if err := s.d.Messages.Update(ctx, m); err != nil {
		return nil, err
	}
	s.tell(ctx, m, domain.MessageEdited{Message: *m})
	return m, nil
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	m, err := s.message(ctx, actor, id)
	if err != nil {
		return err
	}
	hadFile := m.File != nil
	if err := m.Delete(domain.UserID(actor), s.d.Now()); err != nil {
		return err
	}
	if err := s.d.Messages.Update(ctx, m); err != nil {
		return err
	}
	if hadFile && s.d.Blobs != nil {
		_ = s.d.Blobs.Delete(ctx, string(m.ID)) // the message no longer points at it either way
	}
	s.tell(ctx, m, domain.MessageDeleted{Message: *m})
	return nil
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
	now := s.d.Now()
	if err := s.d.Conversations.MarkRead(ctx, c.ID, domain.UserID(actor), now); err != nil {
		return err
	}
	if c.Kind == domain.KindDM {
		s.d.Notifier.Notify(ctx, others(c, domain.UserID(actor)), domain.Read{Conversation: c.ID, User: domain.UserID(actor), At: now})
	}
	return nil
}

// others are the members of c but u.
func others(c *domain.Conversation, u domain.UserID) []domain.UserID {
	out := make([]domain.UserID, 0, len(c.Members))
	for _, m := range c.Members {
		if m != u {
			out = append(out, m)
		}
	}
	return out
}

// Receipts are how far each member of a DM has read it (S68); empty for channels.
func (s *Service) Receipts(ctx context.Context, actor, conv string) (map[string]time.Time, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	if c.Kind != domain.KindDM {
		return out, nil
	}
	cursors, err := s.d.Conversations.ReadCursors(ctx, c.ID)
	for u, at := range cursors {
		out[string(u)] = at
	}
	return out, err
}

// Typing tells the other members that actor is writing.
func (s *Service) Typing(ctx context.Context, actor, conv string) error {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return domain.ErrNotMember
	}
	others := make([]domain.UserID, 0, len(c.Members))
	for _, m := range c.Members {
		if m != domain.UserID(actor) {
			others = append(others, m)
		}
	}
	s.d.Notifier.Notify(ctx, others, domain.Typing{Conversation: c.ID, User: domain.UserID(actor)})
	return nil
}

// member loads a message and its conversation, which actor must belong to.
func (s *Service) asMember(ctx context.Context, actor, id string) (*domain.Message, *domain.Conversation, error) {
	m, err := s.message(ctx, actor, id)
	if err != nil {
		return nil, nil, err
	}
	c, err := s.d.Conversations.ByID(ctx, m.Conversation)
	if err != nil {
		return nil, nil, err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return nil, nil, domain.ErrNotMember
	}
	if err := c.Writable(); err != nil {
		return nil, nil, err
	}
	return m, c, nil
}

// Reply answers a top-level message in its thread.
func (s *Service) Reply(ctx context.Context, actor, parent, body string) (*domain.Message, error) {
	p, c, err := s.asMember(ctx, actor, parent)
	if err != nil {
		return nil, err
	}
	m, err := domain.NewReply(domain.MessageID(s.d.NewID()), p, domain.UserID(actor), body, s.d.Now())
	if err != nil {
		return nil, err
	}
	if err := s.d.Messages.Post(ctx, m); err != nil {
		return nil, err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *m})
	return m, nil
}

// Replies lists a thread, oldest first.
func (s *Service) Replies(ctx context.Context, actor, parent string) ([]*domain.Message, error) {
	p, err := s.message(ctx, actor, parent)
	if err != nil {
		return nil, err
	}
	return s.d.Messages.Replies(ctx, p.ID)
}

// React toggles actor's emoji on a message.
func (s *Service) React(ctx context.Context, actor, id, emoji string) (*domain.Message, error) {
	m, c, err := s.asMember(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if _, err := m.React(domain.UserID(actor), emoji); err != nil {
		return nil, err
	}
	if err := s.d.Messages.Update(ctx, m); err != nil {
		return nil, err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessageReacted{Message: *m})
	return m, nil
}

// WithIssues enables issue previews.
func (s *Service) WithIssues(i Issues) *Service {
	s.d.Issues = i
	return s
}

// Previews resolves the issue keys written in messages to the issues actor may browse.
func (s *Service) Previews(ctx context.Context, actor string, msgs []*domain.Message) (map[domain.MessageID][]IssuePreview, error) {
	out := map[domain.MessageID][]IssuePreview{}
	if s.d.Issues == nil {
		return out, nil
	}
	type seen struct {
		p  IssuePreview
		ok bool
	}
	cache := map[string]seen{}
	for _, m := range msgs {
		for _, key := range domain.IssueKeys(m.Body) {
			c, done := cache[key]
			if !done {
				p, ok, err := s.d.Issues.Preview(ctx, actor, key)
				if err != nil {
					return nil, err
				}
				c = seen{p, ok}
				cache[key] = c
			}
			if c.ok {
				out[m.ID] = append(out[m.ID], c.p)
			}
		}
	}
	return out, nil
}

// OnCallEvent writes "started a huddle" / "the huddle ended" into the call's conversation (call.started,
// call.ended). Redelivered events find the message already there.
func (s *Service) OnCallEvent(ctx context.Context, m outbox.Message) error {
	var e struct {
		Call         string `json:"call_id"`
		Conversation string `json:"conversation_id"`
		By           string `json:"by"`
		Seconds      int    `json:"seconds"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		return nil // a poison message must not block the relay
	}
	c, err := s.d.Conversations.ByID(ctx, domain.ConversationID(e.Conversation))
	if errors.Is(err, domain.ErrConversationNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	kind, text := domain.KindHuddleStarted, "started a huddle"
	if m.Name == "call.ended" {
		kind, text = domain.KindHuddleEnded, fmt.Sprintf("The huddle ended after %d min", max(1, (e.Seconds+30)/60))
	}
	msg := domain.NewSystemMessage(domain.MessageID(s.d.NewID()), c.ID, domain.UserID(e.By), kind, "call:"+e.Call+":"+string(kind), text, s.d.Now())
	if err := s.d.Messages.Post(ctx, msg); errors.Is(err, domain.ErrDuplicateRef) {
		return nil
	} else if err != nil {
		return err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *msg})
	return nil
}

func (s *Service) pin(ctx context.Context, actor, id string, on bool) (*domain.Message, error) {
	m, c, err := s.asMember(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if on {
		if err := m.Pin(domain.UserID(actor), s.d.Now()); err != nil {
			return nil, err
		}
	} else {
		m.Unpin()
	}
	if err := s.d.Messages.Update(ctx, m); err != nil {
		return nil, err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessagePinned{Message: *m})
	return m, nil
}

// Pin keeps a message at hand for everyone in the conversation (members only).
func (s *Service) Pin(ctx context.Context, actor, id string) (*domain.Message, error) {
	return s.pin(ctx, actor, id, true)
}

func (s *Service) Unpin(ctx context.Context, actor, id string) (*domain.Message, error) {
	return s.pin(ctx, actor, id, false)
}

// Pins lists a conversation's pinned messages, most recently pinned first.
func (s *Service) Pins(ctx context.Context, actor, conv string) ([]*domain.Message, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	return s.d.Messages.Pinned(ctx, c.ID)
}

// SearchLimit bounds search results.
const SearchLimit = 50

// Search finds messages containing every word of query in the conversations actor may read.
func (s *Service) Search(ctx context.Context, actor, query string) ([]*domain.Message, error) {
	if len([]rune(strings.TrimSpace(query))) < 2 {
		return nil, ErrQueryTooShort
	}
	convs, err := s.d.Conversations.Visible(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor))
	if err != nil {
		return nil, err
	}
	ids := make([]domain.ConversationID, len(convs))
	for i, c := range convs {
		ids[i] = c.ID
	}
	return s.d.Messages.Search(ctx, ids, query, SearchLimit)
}

type counter struct {
	r io.Reader
	n int64
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Upload stores a file and posts it to the conversation (members only). Its type is detected from the
// content, never taken from the client; files over MaxFile are refused and leave nothing behind.
func (s *Service) Upload(ctx context.Context, actor, conv, name string, body io.Reader) (*domain.Message, error) {
	return s.upload(ctx, actor, conv, name, body, false)
}

// UploadRecording posts a huddle recording (a video up to MaxRecording) to the conversation.
func (s *Service) UploadRecording(ctx context.Context, actor, conv, name string, body io.Reader) (*domain.Message, error) {
	return s.upload(ctx, actor, conv, name, body, true)
}

func (s *Service) upload(ctx context.Context, actor, conv, name string, body io.Reader, recording bool) (*domain.Message, error) {
	if s.d.Blobs == nil {
		return nil, domain.ErrInvalidFile
	}
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return nil, domain.ErrNotMember
	}
	if err := c.Writable(); err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(body, 512)
	head, _ := br.Peek(512)
	typ, _, _ := strings.Cut(http.DetectContentType(head), ";")
	id := domain.MessageID(s.d.NewID())
	limit := int64(domain.MaxFile)
	if recording {
		limit = domain.MaxRecording
	}
	cr := &counter{r: io.LimitReader(br, limit+1)}
	if err := s.d.Blobs.Put(ctx, string(id), cr); err != nil {
		return nil, err
	}
	now := s.d.Now()
	newMessage := domain.NewFileMessage
	if recording {
		newMessage = domain.NewRecordingMessage
	}
	m, err := newMessage(id, c.ID, domain.UserID(actor), domain.File{Name: name, Type: typ, Size: cr.n}, now)
	if err == nil {
		err = s.d.Messages.Post(ctx, m)
	}
	if err != nil {
		_ = s.d.Blobs.Delete(ctx, string(id))
		return nil, err
	}
	s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *m})
	return m, s.d.Conversations.MarkRead(ctx, c.ID, domain.UserID(actor), now)
}

// Download opens a message's file for someone who may read the conversation.
func (s *Service) Download(ctx context.Context, actor, id string) (domain.File, io.ReadCloser, error) {
	m, err := s.message(ctx, actor, id)
	if err != nil {
		return domain.File{}, nil, err
	}
	if m.File == nil || s.d.Blobs == nil {
		return domain.File{}, nil, domain.ErrMessageNotFound
	}
	r, err := s.d.Blobs.Open(ctx, string(m.ID))
	if err != nil {
		return domain.File{}, nil, domain.ErrMessageNotFound
	}
	return *m.File, r, nil
}

// SavedItem is a save with its message.
type SavedItem struct {
	Saved   *domain.Saved
	Message *domain.Message
}

// readableMessage is a message in a conversation actor may read.
func (s *Service) readableMessage(ctx context.Context, actor, id string) (*domain.Message, error) {
	m, err := s.d.Messages.ByID(ctx, domain.MessageID(id))
	if err != nil {
		return nil, err
	}
	if _, err := s.readable(ctx, actor, string(m.Conversation)); err != nil {
		return nil, domain.ErrMessageNotFound
	}
	return m, nil
}

// SaveMessage saves a message for later (S65); a non-zero remindAt (in the future) reminds the actor then.
func (s *Service) SaveMessage(ctx context.Context, actor, id string, remindAt time.Time) (*domain.Saved, error) {
	m, err := s.readableMessage(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	if !remindAt.IsZero() && !remindAt.After(now) {
		return nil, domain.ErrInvalidReminder
	}
	sv := domain.Save(domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor), *m, remindAt.UTC(), now)
	return sv, s.d.Saved.Put(ctx, sv)
}

func (s *Service) Unsave(ctx context.Context, actor, id string) error {
	return s.d.Saved.Remove(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor), domain.MessageID(id))
}

// SavedFor lists the actor's saves whose messages they may still read, most recently saved first.
func (s *Service) SavedFor(ctx context.Context, actor string) ([]SavedItem, error) {
	list, err := s.d.Saved.For(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor))
	if err != nil {
		return nil, err
	}
	out := []SavedItem{}
	for _, sv := range list {
		if m, err := s.readableMessage(ctx, actor, string(sv.Message)); err == nil {
			out = append(out, SavedItem{Saved: sv, Message: m})
		}
	}
	return out, nil
}

// RemindDue sends the due reminders of every workspace (once each, across replicas): saved messages and
// personal reminders.
func (s *Service) RemindDue(ctx context.Context) (int, error) {
	personal, err := s.remindPersonal(ctx)
	if err != nil || s.d.Saved == nil {
		return personal, err
	}
	now := s.d.Now()
	due, err := s.d.Saved.Due(ctx, now)
	if err != nil {
		return 0, err
	}
	sent := personal
	for _, sv := range due {
		if !sv.Remind(now) {
			continue
		}
		switch err := s.d.Saved.MarkReminded(tenant.With(ctx, string(sv.Workspace)), sv); {
		case errors.Is(err, domain.ErrAlreadyReminded):
		case err != nil:
			return sent, err
		default:
			sent++
		}
	}
	return sent, nil
}

// SetStatus sets the actor's status in the workspace; empty emoji and text clear it, a zero dndUntil ends DND.
func (s *Service) SetStatus(ctx context.Context, actor, emoji, text string, expires, dndUntil time.Time) (*domain.Status, error) {
	st, err := domain.NewStatus(domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor), emoji, text, expires, dndUntil, s.d.Now())
	if err != nil {
		return nil, err
	}
	return st, s.d.Statuses.Put(ctx, st)
}

// Statuses are the current statuses of users, by id, for a member of the workspace (people without one are left out).
func (s *Service) Statuses(ctx context.Context, actor string, users []string) (map[string]domain.StatusView, error) {
	ws := tenant.Workspace(ctx)
	if ok, err := s.d.Members.IsMember(ctx, ws, actor); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrNotWorkspaceMember
	}
	ids := make([]domain.UserID, len(users))
	for i, u := range users {
		ids[i] = domain.UserID(u)
	}
	list, err := s.d.Statuses.Of(ctx, domain.WorkspaceID(ws), ids)
	if err != nil {
		return nil, err
	}
	now := s.d.Now()
	out := map[string]domain.StatusView{}
	for _, st := range list {
		if v := st.At(now); v.Emoji != "" || v.Text != "" || v.DND {
			out[string(st.User)] = v
		}
	}
	return out, nil
}

// InDND reports whether user is in Do Not Disturb in the request's workspace now.
func (s *Service) InDND(ctx context.Context, user string) (bool, error) {
	if s.d.Statuses == nil {
		return false, nil
	}
	list, err := s.d.Statuses.Of(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), []domain.UserID{domain.UserID(user)})
	if err != nil || len(list) == 0 {
		return false, err
	}
	return list[0].At(s.d.Now()).DND, nil
}

// memberOf is a conversation of the request's workspace actor belongs to.
func (s *Service) memberOf(ctx context.Context, actor, conv string) (*domain.Conversation, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	if !c.IsMember(domain.UserID(actor)) {
		return nil, domain.ErrNotMember
	}
	if err := c.Writable(); err != nil {
		return nil, err
	}
	return c, nil
}

// Schedule writes a message to be posted later (S67); only members write.
func (s *Service) Schedule(ctx context.Context, actor, conv, body string, at time.Time) (*domain.Scheduled, error) {
	c, err := s.memberOf(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	sc, err := domain.NewScheduled(domain.ScheduledID(s.d.NewID()), c.Workspace, c.ID, domain.UserID(actor), body, at, s.d.Now())
	if err != nil {
		return nil, err
	}
	return sc, s.d.Scheduled.Add(ctx, sc)
}

// own is the actor's scheduled message (someone else's looks missing).
func (s *Service) own(ctx context.Context, actor, id string) (*domain.Scheduled, error) {
	sc, err := s.d.Scheduled.ByID(ctx, domain.ScheduledID(id))
	if err != nil {
		return nil, err
	}
	if string(sc.Author) != actor || string(sc.Workspace) != tenant.Workspace(ctx) {
		return nil, domain.ErrScheduledNotFound
	}
	return sc, nil
}

func (s *Service) Reschedule(ctx context.Context, actor, id, body string, at time.Time) (*domain.Scheduled, error) {
	sc, err := s.own(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if err := sc.Reschedule(body, at, s.d.Now()); err != nil {
		return nil, err
	}
	return sc, s.d.Scheduled.Update(ctx, sc)
}

func (s *Service) CancelScheduled(ctx context.Context, actor, id string) error {
	sc, err := s.own(ctx, actor, id)
	if err != nil {
		return err
	}
	return s.d.Scheduled.Delete(ctx, sc.ID)
}

// ScheduledFor lists the actor's scheduled messages, soonest first.
func (s *Service) ScheduledFor(ctx context.Context, actor string) ([]*domain.Scheduled, error) {
	return s.d.Scheduled.Mine(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor))
}

// SendDue posts the scheduled messages whose time has come (each once: its Ref makes a repeat a duplicate). A
// message whose author left the conversation is dropped.
func (s *Service) SendDue(ctx context.Context) (int, error) {
	if s.d.Scheduled == nil {
		return 0, nil
	}
	now := s.d.Now()
	due, err := s.d.Scheduled.Due(ctx, now)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, sc := range due {
		wctx := tenant.With(ctx, string(sc.Workspace))
		c, err := s.d.Conversations.ByID(wctx, sc.Conversation)
		if errors.Is(err, domain.ErrConversationNotFound) || (err == nil && (!c.IsMember(sc.Author) || c.Archived())) {
			if err := s.d.Scheduled.Delete(wctx, sc.ID); err != nil {
				return sent, err
			}
			continue
		}
		if err != nil {
			return sent, err
		}
		m := sc.Message(domain.MessageID(s.d.NewID()), now)
		switch err := s.d.Messages.Post(wctx, m); {
		case errors.Is(err, domain.ErrDuplicateRef): // another replica sent it
		case err != nil:
			return sent, err
		default:
			sent++
			s.d.Notifier.Notify(wctx, c.Members, domain.MessagePosted{Message: *m})
		}
		if err := s.d.Scheduled.Delete(wctx, sc.ID); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// Remind sets a personal reminder (/remind me …) in a conversation the actor may read.
func (s *Service) Remind(ctx context.Context, actor, conv, text string, at time.Time) (*domain.Reminder, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	r, err := domain.NewReminder(domain.ReminderID(s.d.NewID()), c.Workspace, domain.UserID(actor), c.ID, text, at, s.d.Now())
	if err != nil {
		return nil, err
	}
	return r, s.d.Reminders.Add(ctx, r)
}

func (s *Service) remindPersonal(ctx context.Context) (int, error) {
	if s.d.Reminders == nil {
		return 0, nil
	}
	now := s.d.Now()
	due, err := s.d.Reminders.Due(ctx, now)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, r := range due {
		if !r.Remind(now) {
			continue
		}
		switch err := s.d.Reminders.MarkReminded(tenant.With(ctx, string(r.Workspace)), r); {
		case errors.Is(err, domain.ErrAlreadyReminded):
		case err != nil:
			return sent, err
		default:
			sent++
		}
	}
	return sent, nil
}

// manageChannel loads a channel the actor may read, applies a change and saves it; the members (and whoever was
// removed) hear conversation.updated.
func (s *Service) manageChannel(ctx context.Context, actor, id string, change func(c *domain.Conversation) error) (*domain.Conversation, error) {
	c, err := s.readable(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	before := slices.Clone(c.Members)
	if err := change(c); err != nil {
		return nil, err
	}
	if err := s.d.Conversations.Update(ctx, c); err != nil {
		return nil, err
	}
	for _, u := range before {
		if !c.IsMember(u) {
			if err := s.d.Conversations.RemoveMember(ctx, c.ID, u); err != nil {
				return nil, err
			}
		}
	}
	s.d.Notifier.Notify(ctx, before, domain.ConversationChanged{Conversation: *c})
	return c, nil
}

func (s *Service) Rename(ctx context.Context, actor, id, name, topic string) (*domain.Conversation, error) {
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error {
		if err := c.Writable(); err != nil {
			return err
		}
		return c.Rename(domain.UserID(actor), name, topic)
	})
}

func (s *Service) Archive(ctx context.Context, actor, id string) (*domain.Conversation, error) {
	now := s.d.Now()
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error { return c.Archive(domain.UserID(actor), now) })
}

func (s *Service) Unarchive(ctx context.Context, actor, id string) (*domain.Conversation, error) {
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error { return c.Unarchive(domain.UserID(actor)) })
}

func (s *Service) MakeAdmin(ctx context.Context, actor, id, user string) (*domain.Conversation, error) {
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error { return c.MakeAdmin(domain.UserID(actor), domain.UserID(user)) })
}

func (s *Service) RemoveAdmin(ctx context.Context, actor, id, user string) (*domain.Conversation, error) {
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error { return c.RemoveAdmin(domain.UserID(actor), domain.UserID(user)) })
}

// RemoveMember: an owner or admin takes someone out of the channel.
func (s *Service) RemoveMember(ctx context.Context, actor, id, user string) (*domain.Conversation, error) {
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error {
		if err := c.Writable(); err != nil {
			return err
		}
		return c.Remove(domain.UserID(actor), domain.UserID(user))
	})
}

// workspaceMember: only people of the request's workspace see or change its groups.
func (s *Service) workspaceMember(ctx context.Context, actor string) (string, error) {
	ws := tenant.Workspace(ctx)
	ok, err := s.d.Members.IsMember(ctx, ws, actor)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotWorkspaceMember
	}
	return ws, nil
}

func userIDs(us []string) []domain.UserID {
	out := make([]domain.UserID, len(us))
	for i, u := range us {
		out[i] = domain.UserID(u)
	}
	return out
}

// CreateGroup: anyone in the workspace makes a group of people in it (S70).
func (s *Service) CreateGroup(ctx context.Context, actor, handle, name string, people []string) (*domain.Group, error) {
	ws, err := s.workspaceMember(ctx, actor)
	if err != nil {
		return nil, err
	}
	if err := s.requireMembers(ctx, ws, people); err != nil {
		return nil, err
	}
	g, err := domain.NewGroup(domain.GroupID(s.d.NewID()), domain.WorkspaceID(ws), handle, name, userIDs(people), domain.UserID(actor), s.d.Now())
	if err != nil {
		return nil, err
	}
	return g, s.d.Groups.Save(ctx, g)
}

func (s *Service) group(ctx context.Context, actor, id string) (*domain.Group, error) {
	ws, err := s.workspaceMember(ctx, actor)
	if err != nil {
		return nil, err
	}
	g, err := s.d.Groups.ByID(ctx, domain.GroupID(id))
	if err != nil {
		return nil, err
	}
	if string(g.Workspace) != ws {
		return nil, domain.ErrGroupNotFound
	}
	return g, nil
}

// UpdateGroup: anyone in the workspace renames a group or changes its members.
func (s *Service) UpdateGroup(ctx context.Context, actor, id, handle, name string, people []string) (*domain.Group, error) {
	g, err := s.group(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireMembers(ctx, string(g.Workspace), people); err != nil {
		return nil, err
	}
	if err := g.Change(handle, name, userIDs(people)); err != nil {
		return nil, err
	}
	return g, s.d.Groups.Save(ctx, g)
}

// DeleteGroup: only whoever made the group.
func (s *Service) DeleteGroup(ctx context.Context, actor, id string) error {
	g, err := s.group(ctx, actor, id)
	if err != nil {
		return err
	}
	if string(g.CreatedBy) != actor {
		return domain.ErrNotOwner
	}
	return s.d.Groups.Delete(ctx, g.ID)
}

func (s *Service) Groups(ctx context.Context, actor string) ([]*domain.Group, error) {
	ws, err := s.workspaceMember(ctx, actor)
	if err != nil {
		return nil, err
	}
	return s.d.Groups.In(ctx, domain.WorkspaceID(ws))
}

// GroupMembers are the people of the request's workspace group @handle (none when there is no such group).
func (s *Service) GroupMembers(ctx context.Context, handle string) ([]string, error) {
	if s.d.Groups == nil {
		return nil, nil
	}
	g, err := s.d.Groups.ByHandle(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), handle)
	if errors.Is(err, domain.ErrGroupNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, len(g.Members))
	for i, m := range g.Members {
		out[i] = string(m)
	}
	return out, nil
}

// SetPref sets the actor's notification level and sidebar section for a conversation they may read (S71).
func (s *Service) SetPref(ctx context.Context, actor, conv string, level domain.Level, section string) (*domain.Pref, error) {
	c, err := s.readable(ctx, actor, conv)
	if err != nil {
		return nil, err
	}
	p, err := domain.NewPref(c.Workspace, domain.UserID(actor), c.ID, level, section)
	if err != nil {
		return nil, err
	}
	return p, s.d.Prefs.Put(ctx, p)
}

func (s *Service) PrefsFor(ctx context.Context, actor string) ([]*domain.Pref, error) {
	return s.d.Prefs.Mine(ctx, domain.WorkspaceID(tenant.Workspace(ctx)), domain.UserID(actor))
}

// Followers are the members of a conversation who want every message.
func (s *Service) Followers(ctx context.Context, conv string) ([]string, error) {
	if s.d.Prefs == nil {
		return nil, nil
	}
	c, err := s.d.Conversations.ByID(ctx, domain.ConversationID(conv))
	if err != nil {
		return nil, err
	}
	prefs, err := s.d.Prefs.Of(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range prefs {
		if p.Level == domain.LevelAll && c.IsMember(p.User) {
			out = append(out, string(p.User))
		}
	}
	slices.Sort(out)
	return out, nil
}

// Muted reports whether user muted the conversation.
func (s *Service) Muted(ctx context.Context, user, conv string) (bool, error) {
	if s.d.Prefs == nil {
		return false, nil
	}
	prefs, err := s.d.Prefs.Of(ctx, domain.ConversationID(conv))
	if err != nil {
		return false, err
	}
	for _, p := range prefs {
		if string(p.User) == user {
			return p.Level == domain.LevelMuted, nil
		}
	}
	return false, nil
}

// LinkProject posts a project's news into the channel (S73); "" unlinks. Owner/admins who may browse it.
func (s *Service) LinkProject(ctx context.Context, actor, id, key string) (*domain.Conversation, error) {
	if key != "" {
		if s.d.Projects == nil {
			return nil, ErrCannotBrowse
		}
		ok, err := s.d.Projects.CanBrowse(ctx, actor, strings.ToUpper(key))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrCannotBrowse
		}
	}
	return s.manageChannel(ctx, actor, id, func(c *domain.Conversation) error { return c.LinkProject(domain.UserID(actor), key) })
}

// OnProjectEvent posts issue.created, issue.transitioned, sprint.started and sprint.completed into the channels
// linked to the project (once per event and channel). Issues are named by key only: readers who may browse them
// see the title through the usual preview.
func (s *Service) OnProjectEvent(ctx context.Context, m outbox.Message) error {
	var e struct {
		Key      string `json:"key"`
		To       string `json:"to"`
		Project  string `json:"project"`
		Reporter string `json:"reporter"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		return nil // a poison message must not block the relay
	}
	project, by, text := e.Project, m.Actor, ""
	if e.Key != "" {
		project = e.Key[:max(0, strings.LastIndex(e.Key, "-"))]
	}
	switch m.Name {
	case "issue.created":
		text = "created " + e.Key
		if e.Reporter != "" {
			by = e.Reporter
		}
	case "issue.transitioned":
		text = "moved " + e.Key + " to " + e.To
	case "sprint.started":
		text = "started a sprint in " + project
	case "sprint.completed":
		text = "completed a sprint in " + project
	default:
		return nil
	}
	if project == "" {
		return nil
	}
	channels, err := s.d.Conversations.ByProject(ctx, domain.WorkspaceID(m.Workspace), project)
	if err != nil {
		return err
	}
	for _, c := range channels {
		if c.Archived() {
			continue
		}
		ref := fmt.Sprintf("outbox:%d:%s", m.ID, c.ID)
		msg := domain.NewSystemMessage(domain.MessageID(s.d.NewID()), c.ID, domain.UserID(by), domain.KindProjectEvent, ref, text, s.d.Now())
		if err := s.d.Messages.Post(ctx, msg); errors.Is(err, domain.ErrDuplicateRef) {
			continue
		} else if err != nil {
			return err
		}
		s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *msg})
	}
	return nil
}

// OnRecordingPosted transcribes a huddle recording into its thread when a speech-to-text service is configured
// (chat.message_posted of kind huddle_recording); once per recording.
func (s *Service) OnRecordingPosted(ctx context.Context, m outbox.Message) error {
	var e domain.MessageWritten
	if err := json.Unmarshal(m.Payload, &e); err != nil || e.Kind != domain.KindHuddleRecording || s.d.Transcriber == nil || s.d.Blobs == nil {
		return nil
	}
	rec, err := s.d.Messages.ByID(ctx, e.ID)
	if errors.Is(err, domain.ErrMessageNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	replies, err := s.d.Messages.Replies(ctx, rec.ID)
	if err != nil {
		return err
	}
	for _, r := range replies {
		if r.Kind == domain.KindTranscript {
			return nil // done already (redelivery)
		}
	}
	if rec.Deleted || rec.File == nil {
		return nil
	}
	audio, err := s.d.Blobs.Open(ctx, string(rec.ID))
	if err != nil {
		return err
	}
	defer audio.Close()
	text, err := s.d.Transcriber.Transcribe(ctx, rec.File.Name, audio)
	if err != nil {
		return err // retried by the relay
	}
	t := domain.NewTranscript(domain.MessageID(s.d.NewID()), rec, text, s.d.Now())
	if err := s.d.Messages.Post(ctx, t); errors.Is(err, domain.ErrDuplicateRef) {
		return nil
	} else if err != nil {
		return err
	}
	c, err := s.d.Conversations.ByID(ctx, rec.Conversation)
	if err == nil {
		s.d.Notifier.Notify(ctx, c.Members, domain.MessagePosted{Message: *t})
	}
	return nil
}
