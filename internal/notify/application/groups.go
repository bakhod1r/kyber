package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrForbidden      = errors.New("only project admins can connect Telegram groups")
	ErrNotFound       = errors.New("project not found")
	ErrGroupsDisabled = errors.New("no Telegram bot is configured")
)

// Admins checks project administration (ACL to the Access context): ErrNotFound for outsiders,
// ErrForbidden for members who are not admins.
type Admins interface {
	Administer(ctx context.Context, actor, project string) error
}

// Messenger replies in a Telegram chat (the bot).
type Messenger interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

// Lookups names things for group messages (ACL to Issue Tracking and Agile).
type Lookups interface {
	StatusName(ctx context.Context, project, status string) (string, error)
	SprintName(ctx context.Context, sprint string) (project, name string, err error)
}

func (s *Service) groupsEnabled() bool {
	return s.d.Groups != nil && s.d.Bot != nil && s.d.BotName != "" && s.d.Admins != nil
}

// StartGroupLink returns a startgroup deep link for a project admin; opening it lets them pick a
// Telegram group, adds the bot and sends it the one-time token.
func (s *Service) StartGroupLink(ctx context.Context, actor, project string) (string, time.Time, error) {
	if s.d.Admins == nil {
		return "", time.Time{}, ErrGroupsDisabled
	}
	if err := s.d.Admins.Administer(ctx, actor, project); err != nil {
		return "", time.Time{}, err
	}
	if !s.groupsEnabled() {
		return "", time.Time{}, ErrGroupsDisabled
	}
	raw, tok := domain.NewGroupToken(tenant.Workspace(ctx), project, domain.UserID(actor), s.d.Now())
	if err := s.d.Groups.CreateGroupToken(ctx, tok); err != nil {
		return "", time.Time{}, err
	}
	return "https://t.me/" + url.PathEscape(s.d.BotName) + "?startgroup=" + url.QueryEscape(raw), tok.ExpiresAt, nil
}

// OnGroupStart handles "/start g_<token>" in a group: the group starts getting the project's events.
// The admin's right is checked again, now: a revoked admin's old link does nothing.
func (s *Service) OnGroupStart(ctx context.Context, payload, chatID, title string) error {
	if !s.groupsEnabled() {
		return ErrGroupsDisabled
	}
	tok, err := s.d.Groups.ConsumeGroupToken(ctx, domain.HashGroupToken(payload), s.d.Now())
	if err != nil {
		if errors.Is(err, domain.ErrGroupTokenInvalid) {
			_ = s.d.Bot.SendMessage(ctx, chatID, "This link is not valid any more. A project admin can get a new one in Kyber: "+
				"project → Notifications → Connect a Telegram group.")
		}
		return err
	}
	ctx = tenant.With(ctx, tok.Workspace)
	if err := s.d.Admins.Administer(ctx, string(tok.User), tok.Project); err != nil {
		return err
	}
	if err := s.d.Groups.LinkChat(ctx, domain.ProjectChat{Workspace: tok.Workspace, Project: tok.Project, ChatID: chatID,
		Title: title, LinkedBy: tok.User, LinkedAt: s.d.Now()}); err != nil {
		return err
	}
	return s.d.Bot.SendMessage(ctx, chatID, fmt.Sprintf("✅ This group now gets %s updates from Kyber: new issues, "+
		"status changes and sprints.", tok.Project))
}

// GroupChats lists the project's groups to its admins. Authorization comes first, even when no
// bot is configured, so outsiders always get ErrNotFound.
func (s *Service) GroupChats(ctx context.Context, actor, project string) ([]domain.ProjectChat, error) {
	if s.d.Admins == nil {
		return nil, ErrGroupsDisabled
	}
	if err := s.d.Admins.Administer(ctx, actor, project); err != nil {
		return nil, err
	}
	if !s.groupsEnabled() {
		return []domain.ProjectChat{}, nil
	}
	return s.d.Groups.ChatsOf(ctx, project)
}

func (s *Service) UnlinkGroup(ctx context.Context, actor, project, chatID string) error {
	if s.d.Admins == nil {
		return ErrGroupsDisabled
	}
	if err := s.d.Admins.Administer(ctx, actor, project); err != nil {
		return err
	}
	if !s.groupsEnabled() {
		return ErrGroupsDisabled
	}
	return s.d.Groups.UnlinkChat(ctx, project, chatID)
}

// OnProjectEvent posts issue.created, status changes (issue.changed) and sprint.started/completed
// to the project's Telegram groups through the delivery queue (retries, no duplicates).
func (s *Service) OnProjectEvent(ctx context.Context, m outbox.Message) error {
	if !s.groupsEnabled() || s.d.Deliveries == nil {
		return nil
	}
	var e struct {
		ID, Key, Title, Reporter, By, Project, Author, Excerpt string
		Items                                                  []fieldChange
		Start, End                                             time.Time
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil {
		s.d.Log.ErrorContext(ctx, "notify: skipping malformed event", "id", m.ID, "name", m.Name)
		return nil
	}
	ev, project, err := s.projectEvent(ctx, m.Name, e.ID, e.Key, e.Title, e.Reporter, e.By, e.Items)
	if m.Name == "announcement.published" && e.Project != "" {
		var actor string
		actor, err = s.d.Users.DisplayName(ctx, e.Author)
		ev, project = domain.ProjectEvent{Kind: domain.EventAnnouncement, Actor: actor, Title: e.Title, Text: e.Excerpt}, e.Project
	}
	if strings.HasPrefix(m.Name, "meeting.") && e.Project != "" {
		ev, project = domain.ProjectEvent{Kind: domain.EventMeeting, Title: e.Title, Text: domain.MeetingText(m.Name, e.Start, e.End, "")}, e.Project
	}
	if err != nil || project == "" {
		return err
	}
	chats, err := s.d.Groups.ChatsOf(ctx, project)
	if err != nil {
		return err
	}
	text := ev.Message(fmt.Sprintf("%s/projects/%s", s.d.PublicURL, url.PathEscape(project)))
	switch {
	case ev.Kind == domain.EventMeeting:
		text = ev.Message(s.d.PublicURL + "/meetings")
	case ev.Kind == domain.EventAnnouncement:
		text = ev.Message(announcementLink(s.d.PublicURL, project))
	case ev.IssueKey != "":
		text = ev.Message(fmt.Sprintf("%s/projects/%s?issue=%s", s.d.PublicURL, url.PathEscape(project), url.QueryEscape(ev.IssueKey)))
	}
	for _, c := range chats {
		d, err := domain.NewDelivery(s.d.NewID(), c.Workspace, domain.ChannelTelegram, c.ChatID, "", text,
			fmt.Sprintf("%d/group/%s", m.ID, c.ChatID), s.d.Now())
		if err != nil {
			return err
		}
		if err := s.d.Deliveries.Enqueue(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// projectEvent builds the event; an empty project means "nothing to post".
func (s *Service) projectEvent(ctx context.Context, name, id, key, title, reporter, by string, items []fieldChange) (domain.ProjectEvent, string, error) {
	switch name {
	case "sprint.started", "sprint.completed":
		project, sprint, err := s.d.Lookups.SprintName(ctx, id)
		kind := domain.EventSprintStarted
		if name == "sprint.completed" {
			kind = domain.EventSprintCompleted
		}
		return domain.ProjectEvent{Kind: kind, Sprint: sprint}, project, err
	case "issue.created":
		actor, err := s.d.Users.DisplayName(ctx, reporter)
		return domain.ProjectEvent{Kind: domain.EventIssueCreated, Actor: actor, IssueKey: key, Title: title}, projectOf(key), err
	case "issue.changed":
		for _, it := range items {
			if it.Field != "status" {
				continue
			}
			info, err := s.d.Issues.Lookup(ctx, key)
			if errors.Is(err, ErrIssueGone) || err == nil && info.Confidential {
				return domain.ProjectEvent{}, "", nil
			}
			if err != nil {
				return domain.ProjectEvent{}, "", err
			}
			actor, err := s.d.Users.DisplayName(ctx, by)
			if err != nil {
				return domain.ProjectEvent{}, "", err
			}
			from, err := s.d.Lookups.StatusName(ctx, info.Project, it.From)
			if err != nil {
				return domain.ProjectEvent{}, "", err
			}
			to, err := s.d.Lookups.StatusName(ctx, info.Project, it.To)
			return domain.ProjectEvent{Kind: domain.EventStatusChanged, Actor: actor, IssueKey: key, Title: info.Title, From: from, To: to},
				info.Project, err
		}
	}
	return domain.ProjectEvent{}, "", nil
}

// fieldChange mirrors the items of issue.changed (a published event contract).
type fieldChange struct{ Field, From, To string }

func projectOf(key string) string {
	if i := strings.LastIndexByte(key, '-'); i > 0 {
		return key[:i]
	}
	return ""
}
