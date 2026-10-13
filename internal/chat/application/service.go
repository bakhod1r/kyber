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
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var ErrQueryTooShort = errors.New("search for at least 2 characters")

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
	Saved         domain.SavedMessages // optional: save for later (S65)
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
	return s.d.Conversations.MarkRead(ctx, c.ID, domain.UserID(actor), s.d.Now())
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

// RemindDue sends the due reminders of every workspace (once each, across replicas).
func (s *Service) RemindDue(ctx context.Context) (int, error) {
	if s.d.Saved == nil {
		return 0, nil
	}
	now := s.d.Now()
	due, err := s.d.Saved.Due(ctx, now)
	if err != nil {
		return 0, err
	}
	sent := 0
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
