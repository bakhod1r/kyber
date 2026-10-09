// Package app holds the Notification context: outbox event handlers and inbox queries.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// ErrIssueGone is returned by Issues.Lookup when the issue no longer exists.
var ErrIssueGone = errors.New("issue no longer exists")

type IssueInfo struct {
	Project, Title     string
	Reporter, Assignee string
}

// Ports to the other contexts (anti-corruption layers live in the adapters).
type (
	Issues interface {
		Lookup(ctx context.Context, issueKey string) (IssueInfo, error)
	}
	Members interface {
		IsMember(ctx context.Context, project, user string) (bool, error)
	}
	Users interface {
		UserIDByEmail(ctx context.Context, email string) (id string, ok bool, err error)
		DisplayName(ctx context.Context, id string) (string, error)
	}
)

type Deps struct {
	Repo    domain.Repository
	Issues  Issues
	Members Members
	Users   Users
	NewID   func() string
	Now     func() time.Time
	Log     *slog.Logger
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

const excerptLen = 140

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
	recipients := domain.CommentRecipients(domain.UserID(e.Author), domain.UserID(info.Reporter), domain.UserID(info.Assignee), mentioned)
	return s.deliverTo(ctx, m.ID, e.IssueKey, info, e.Author, excerpt(e.Body), recipients)
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

// deliverTo stores one notification per recipient who is still a project member.
func (s *Service) deliverTo(ctx context.Context, event int64, issueKey string, info IssueInfo, actor, text string, recipients map[domain.UserID]domain.Kind) error {
	if len(recipients) == 0 {
		return nil
	}
	actorName, err := s.d.Users.DisplayName(ctx, actor)
	if err != nil {
		return err
	}
	for u, kind := range recipients {
		ok, err := s.d.Members.IsMember(ctx, info.Project, string(u))
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		n := domain.Notification{
			ID: domain.NotificationID(s.d.NewID()), Recipient: u, Kind: kind, IssueKey: issueKey,
			IssueTitle: info.Title, ActorName: actorName, Excerpt: text, SourceEvent: event, CreatedAt: s.d.Now(),
		}
		if err := s.d.Repo.Add(ctx, n); err != nil {
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
func (s *Service) List(ctx context.Context, user string, unreadOnly bool) ([]domain.Notification, int, error) {
	items, err := s.d.Repo.ListFor(ctx, domain.UserID(user), unreadOnly, pageSize)
	if err != nil {
		return nil, 0, err
	}
	unread, err := s.d.Repo.UnreadCount(ctx, domain.UserID(user))
	return items, unread, err
}

func (s *Service) MarkRead(ctx context.Context, user, id string) error {
	return s.d.Repo.MarkRead(ctx, domain.UserID(user), domain.NotificationID(id), s.d.Now())
}

func (s *Service) MarkAllRead(ctx context.Context, user string) error {
	return s.d.Repo.MarkAllRead(ctx, domain.UserID(user), s.d.Now())
}
