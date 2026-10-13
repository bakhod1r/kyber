// Package app holds the Notification context: outbox event handlers and inbox queries.
package application

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// ErrIssueGone is returned by Issues.Lookup when the issue no longer exists.
var ErrIssueGone = errors.New("issue no longer exists")

type IssueInfo struct {
	Project, Title     string
	Reporter, Assignee string
	Confidential       bool // has a security level: never posted to Telegram groups
}

// Ports to the other contexts (anti-corruption layers live in the adapters).
type (
	Issues interface {
		Lookup(ctx context.Context, issueKey string) (IssueInfo, error)
		// Watchers lists the users watching the issue.
		Watchers(ctx context.Context, issueKey string) ([]string, error)
	}
	Members interface {
		// CanSee reports whether the user may still browse the issue (membership and security level).
		CanSee(ctx context.Context, user, issueKey string) (bool, error)
	}
	// Workspaces tells which workspace a project belongs to.
	Workspaces interface {
		WorkspaceOf(ctx context.Context, project string) (string, error)
	}
	// Audience lists who hears an announcement (ACL to Project/Workspace).
	Audience interface {
		ProjectMembers(ctx context.Context, project string) ([]string, error)
		// WorkspaceMembers lists everyone in the request's workspace.
		WorkspaceMembers(ctx context.Context) ([]string, error)
	}
	Users interface {
		UserIDByEmail(ctx context.Context, email string) (id string, ok bool, err error)
		DisplayName(ctx context.Context, id string) (string, error)
	}
)

// Conversations is the port to the Chat context: may user read the conversation, and its name.
type Conversations interface {
	Readable(ctx context.Context, user, conversation string) (name string, ok bool, err error)
}

type Deps struct {
	Conversations Conversations // optional: chat mentions
	Repo          domain.Repository
	Deliveries    domain.DeliveryRepository // optional: email/Telegram copies of notifications
	Prefs         domain.PrefsRepository
	Contacts      Contacts
	Channels      []domain.Channel       // channels configured on this server (SMTP, Telegram bot)
	PublicURL     string                 // base of the links in messages
	Groups        domain.GroupRepository // optional: Telegram groups subscribed to projects
	Admins        Admins
	Bot           Messenger // replies in Telegram when a group is connected
	BotName       string
	Lookups       Lookups
	Issues        Issues
	Members       Members
	Workspaces    Workspaces
	Users         Users
	Audience      Audience
	NewID         func() string
	Now           func() time.Time
	Log           *slog.Logger
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

// WithConversations enables notifications for mentions in chat.
func (s *Service) WithConversations(c Conversations) *Service {
	s.d.Conversations = c
	return s
}

const excerptLen = 140

// announcement mirrors announcement.published (a published event contract).
type announcement struct {
	ID, Workspace, Project, Title, Excerpt, Author string
}

// OnAnnouncement tells the project's members (or the workspace's, without a project) about an
// announcement, through their inbox and their chosen channels; never its author.
func (s *Service) OnAnnouncement(ctx context.Context, m outbox.Message) error {
	var a announcement
	if err := json.Unmarshal(m.Payload, &a); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "name", m.Name)
		return nil
	}
	var audience []string
	var err error
	if a.Project != "" {
		audience, err = s.d.Audience.ProjectMembers(ctx, a.Project)
	} else {
		audience, err = s.d.Audience.WorkspaceMembers(ctx)
	}
	if err != nil {
		return err
	}
	ids := make([]domain.UserID, len(audience))
	for i, u := range audience {
		ids[i] = domain.UserID(u)
	}
	actor, err := s.d.Users.DisplayName(ctx, a.Author)
	if err != nil {
		return err
	}
	for _, u := range domain.AnnouncementRecipients(domain.UserID(a.Author), ids) {
		n := domain.Notification{
			ID: domain.NotificationID(s.d.NewID()), Recipient: u, Workspace: a.Workspace, Kind: domain.KindAnnouncement,
			Project: a.Project, IssueTitle: a.Title, ActorName: actor, Excerpt: a.Excerpt, SourceEvent: m.ID, CreatedAt: s.d.Now(),
		}
		if err := s.d.Repo.Add(ctx, n); err != nil {
			return err
		}
		if err := s.enqueue(ctx, n, a.Project); err != nil {
			return err
		}
	}
	return nil
}

// meeting mirrors the meeting.* events (a published event contract of the Calendar context).
type meeting struct {
	Workspace, Project, Title string
	Start, End                time.Time
	Attendees                 []string
	Organizer, By             string
}

// OnMeetingEvent tells the invited about meeting.scheduled and meeting.cancelled (not whoever did
// it) and reminds them before meeting.reminder's start.
func (s *Service) OnMeetingEvent(ctx context.Context, m outbox.Message) error {
	var e meeting
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "name", m.Name)
		return nil
	}
	actor := cmp.Or(e.Organizer, e.By)
	actorName := ""
	if actor != "" {
		name, err := s.d.Users.DisplayName(ctx, actor)
		if err != nil {
			return err
		}
		actorName = name
	}
	ids := make([]domain.UserID, len(e.Attendees))
	for i, u := range e.Attendees {
		ids[i] = domain.UserID(u)
	}
	for _, u := range domain.AnnouncementRecipients(domain.UserID(actor), ids) {
		n := domain.Notification{
			ID: domain.NotificationID(s.d.NewID()), Recipient: u, Workspace: e.Workspace, Kind: domain.KindMeeting, Project: e.Project,
			IssueTitle: e.Title, ActorName: actorName, Excerpt: domain.MeetingText(m.Name, e.Start, e.End), SourceEvent: m.ID, CreatedAt: s.d.Now(),
		}
		if err := s.d.Repo.Add(ctx, n); err != nil {
			return err
		}
		if err := s.enqueue(ctx, n, e.Project); err != nil {
			return err
		}
	}
	return nil
}

// OnIssueAssigned handles issue.assigned events.
func (s *Service) OnIssueAssigned(ctx context.Context, m outbox.Message) error {
	var e struct{ Key, To, By string }
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "err", err)
		return nil // a poison message must not block the relay forever
	}
	recipients := map[domain.UserID]domain.Kind{}
	for _, u := range domain.AssignmentRecipients(domain.UserID(e.To), domain.UserID(e.By)) {
		recipients[u] = domain.KindAssigned
	}
	return s.deliver(ctx, m.ID, e.Key, e.By, "", recipients)
}

// OnCommentAdded handles comment.added events (commented + mentioned).
func (s *Service) OnCommentAdded(ctx context.Context, m outbox.Message) error {
	var e struct {
		IssueKey string `json:"issue_key"`
		Author   string `json:"author"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "err", err)
		return nil
	}
	info, err := s.d.Issues.Lookup(ctx, e.IssueKey)
	if errors.Is(err, ErrIssueGone) {
		return nil
	}
	if err != nil {
		return err
	}
	var mentioned []domain.UserID
	for _, email := range domain.ParseMentions(e.Body) {
		id, ok, err := s.d.Users.UserIDByEmail(ctx, email)
		if err != nil {
			return err
		}
		if ok {
			mentioned = append(mentioned, domain.UserID(id))
		}
	}
	watchers, err := s.watchers(ctx, e.IssueKey)
	if err != nil {
		return err
	}
	recipients := domain.CommentRecipients(domain.UserID(e.Author), domain.UserID(info.Reporter), domain.UserID(info.Assignee), watchers, mentioned)
	return s.deliverTo(ctx, m.ID, e.IssueKey, info, e.Author, excerpt(e.Body), recipients)
}

func (s *Service) watchers(ctx context.Context, issueKey string) ([]domain.UserID, error) {
	ids, err := s.d.Issues.Watchers(ctx, issueKey)
	out := make([]domain.UserID, len(ids))
	for i, id := range ids {
		out[i] = domain.UserID(id)
	}
	return out, err
}

// OnIssueChanged tells the reporter, assignee and watchers about status changes (issue.changed).
func (s *Service) OnIssueChanged(ctx context.Context, m outbox.Message) error {
	var e struct {
		Key, By string
		Items   []fieldChange
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "err", err)
		return nil
	}
	for _, it := range e.Items {
		if it.Field != "status" {
			continue
		}
		info, err := s.d.Issues.Lookup(ctx, e.Key)
		if errors.Is(err, ErrIssueGone) {
			return nil
		}
		if err != nil {
			return err
		}
		watchers, err := s.watchers(ctx, e.Key)
		if err != nil {
			return err
		}
		from, err := s.d.Lookups.StatusName(ctx, info.Project, it.From)
		if err != nil {
			return err
		}
		to, err := s.d.Lookups.StatusName(ctx, info.Project, it.To)
		if err != nil {
			return err
		}
		recipients := domain.TransitionRecipients(domain.UserID(e.By), domain.UserID(info.Reporter), domain.UserID(info.Assignee), watchers)
		return s.deliverTo(ctx, m.ID, e.Key, info, e.By, from+" → "+to, recipients)
	}
	return nil
}

func (s *Service) deliver(ctx context.Context, event int64, issueKey, actor, text string, recipients map[domain.UserID]domain.Kind) error {
	if len(recipients) == 0 {
		return nil
	}
	info, err := s.d.Issues.Lookup(ctx, issueKey)
	if errors.Is(err, ErrIssueGone) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.deliverTo(ctx, event, issueKey, info, actor, text, recipients)
}

// deliverTo stores one notification per recipient who may still see the issue.
func (s *Service) deliverTo(ctx context.Context, event int64, issueKey string, info IssueInfo, actor, text string, recipients map[domain.UserID]domain.Kind) error {
	if len(recipients) == 0 {
		return nil
	}
	actorName := "Someone" // legacy events (before 0.6) do not record the actor
	if actor != "" {
		name, err := s.d.Users.DisplayName(ctx, actor)
		if err != nil {
			return err
		}
		actorName = name
	}
	ws, err := s.d.Workspaces.WorkspaceOf(ctx, info.Project)
	if err != nil {
		return err
	}
	for u, kind := range recipients {
		ok, err := s.d.Members.CanSee(ctx, string(u), issueKey)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		n := domain.Notification{
			ID: domain.NotificationID(s.d.NewID()), Recipient: u, Workspace: ws, Kind: kind, IssueKey: issueKey,
			IssueTitle: info.Title, ActorName: actorName, Excerpt: text, SourceEvent: event, CreatedAt: s.d.Now(),
		}
		if err := s.d.Repo.Add(ctx, n); err != nil {
			return err
		}
		if err := s.enqueue(ctx, n, info.Project); err != nil {
			return err
		}
	}
	return nil
}

func excerpt(body string) string {
	r := []rune(body)
	if len(r) <= excerptLen {
		return body
	}
	return string(r[:excerptLen-1]) + "…"
}

const pageSize = 50

// List returns the user's newest notifications and their unread count.
// inbox is the user's inbox in the request's workspace.
func inbox(ctx context.Context, user string) domain.Inbox {
	ws, ok := tenant.From(ctx)
	if !ok {
		ws = tenant.Default
	}
	return domain.Inbox{Workspace: ws, User: domain.UserID(user)}
}

func (s *Service) List(ctx context.Context, user string, unreadOnly bool) ([]domain.Notification, int, error) {
	items, err := s.d.Repo.ListFor(ctx, inbox(ctx, user), unreadOnly, pageSize)
	if err != nil {
		return nil, 0, err
	}
	unread, err := s.d.Repo.UnreadCount(ctx, inbox(ctx, user))
	return items, unread, err
}

func (s *Service) MarkRead(ctx context.Context, user, id string) error {
	return s.d.Repo.MarkRead(ctx, inbox(ctx, user), domain.NotificationID(id), s.d.Now())
}

// MarkConversationRead reads the user's chat mentions in a conversation (they opened it).
func (s *Service) MarkConversationRead(ctx context.Context, user, conversation string) error {
	return s.d.Repo.MarkConversationRead(ctx, inbox(ctx, user), conversation, s.d.Now())
}

func (s *Service) MarkAllRead(ctx context.Context, user string) error {
	return s.d.Repo.MarkAllRead(ctx, inbox(ctx, user), s.d.Now())
}

// OnChatMessage tells people mentioned (@email) in a chat message, if they may read it.
func (s *Service) OnChatMessage(ctx context.Context, m outbox.Message) error {
	var e struct {
		Conversation string `json:"conversation_id"`
		Author       string `json:"author_id"`
		Body         string `json:"body"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "err", err)
		return nil
	}
	if s.d.Conversations == nil {
		return nil
	}
	var actorName string
	for _, email := range domain.ParseMentions(e.Body) {
		id, ok, err := s.d.Users.UserIDByEmail(ctx, email)
		if err != nil {
			return err
		}
		if !ok || id == e.Author {
			continue
		}
		name, readable, err := s.d.Conversations.Readable(ctx, id, e.Conversation)
		if err != nil {
			return err
		}
		if !readable {
			continue
		}
		if actorName == "" {
			if actorName, err = s.d.Users.DisplayName(ctx, e.Author); err != nil {
				return err
			}
		}
		n := domain.Notification{
			ID: domain.NotificationID(s.d.NewID()), Recipient: domain.UserID(id), Workspace: tenant.Workspace(ctx),
			Kind: domain.KindMentioned, ConversationID: e.Conversation, ConversationName: name, ActorName: actorName,
			Excerpt: excerpt(e.Body), SourceEvent: m.ID, CreatedAt: s.d.Now(),
		}
		if err := s.d.Repo.Add(ctx, n); err != nil {
			return err
		}
		if err := s.enqueue(ctx, n, ""); err != nil {
			return err
		}
	}
	return nil
}

// OnChatReminder tells the user about a saved message whose reminder is due (chat.reminder), if they may
// still read its conversation.
func (s *Service) OnChatReminder(ctx context.Context, m outbox.Message) error {
	var e struct {
		User         string `json:"user_id"`
		Conversation string `json:"conversation_id"`
		Excerpt      string `json:"excerpt"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "err", err)
		return nil
	}
	if s.d.Conversations == nil {
		return nil
	}
	name, readable, err := s.d.Conversations.Readable(ctx, e.User, e.Conversation)
	if err != nil || !readable {
		return err
	}
	n := domain.Notification{
		ID: domain.NotificationID(s.d.NewID()), Recipient: domain.UserID(e.User), Workspace: tenant.Workspace(ctx),
		Kind: domain.KindReminder, ConversationID: e.Conversation, ConversationName: name, Excerpt: e.Excerpt,
		SourceEvent: m.ID, CreatedAt: s.d.Now(),
	}
	if err := s.d.Repo.Add(ctx, n); err != nil {
		return err
	}
	return s.enqueue(ctx, n, "")
}
