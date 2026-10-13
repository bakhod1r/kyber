package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Telegram groups subscribed to a project: an admin gets a startgroup link
// (t.me/<bot>?startgroup=g_<token>); adding the bot to a group with it links the group.

var (
	ErrGroupTokenInvalid = errors.New("the group link is not valid any more")
	ErrChatNotLinked     = errors.New("this chat is not connected to the project")
)

const GroupTokenTTL = 10 * time.Minute

type GroupToken struct {
	Hash      string // SHA-256 of the raw token; the raw token is never stored
	Workspace string
	Project   string
	User      UserID // the admin who asked for the link
	ExpiresAt time.Time
}

func NewGroupToken(workspace, project string, user UserID, now time.Time) (raw string, t GroupToken) {
	var b [24]byte
	_, _ = rand.Read(b[:])
	raw = "g_" + base64.RawURLEncoding.EncodeToString(b[:])
	return raw, GroupToken{Hash: HashGroupToken(raw), Workspace: workspace, Project: project, User: user, ExpiresAt: now.Add(GroupTokenTTL)}
}

func HashGroupToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ProjectChat is a Telegram group receiving a project's events.
type ProjectChat struct {
	Workspace string
	Project   string
	ChatID    string
	Title     string
	LinkedBy  UserID
	LinkedAt  time.Time
}

// GroupRepository stores group links (chat lists are scoped to the request's workspace).
type GroupRepository interface {
	CreateGroupToken(ctx context.Context, t GroupToken) error
	// ConsumeGroupToken returns the token once (ErrGroupTokenInvalid when unknown, used or expired).
	ConsumeGroupToken(ctx context.Context, hash string, now time.Time) (GroupToken, error)
	// LinkChat connects the chat to the project (idempotent; the title is refreshed).
	LinkChat(ctx context.Context, c ProjectChat) error
	UnlinkChat(ctx context.Context, project, chatID string) error // ErrChatNotLinked
	ChatsOf(ctx context.Context, project string) ([]ProjectChat, error)
}

// ProjectEvent is what a project's groups are told about.
type ProjectEvent struct {
	Kind            EventKind
	Actor           string
	IssueKey, Title string
	From, To        string // status names
	Sprint          string
	Text            string // an announcement's excerpt
}

type EventKind string

const (
	EventIssueCreated    EventKind = "issue_created"
	EventStatusChanged   EventKind = "status_changed"
	EventSprintStarted   EventKind = "sprint_started"
	EventSprintCompleted EventKind = "sprint_completed"
	EventAnnouncement    EventKind = "announcement"
	EventMeeting         EventKind = "meeting"
)

// Message is the Telegram text for the event, ending with a link to the project.
func (e ProjectEvent) Message(link string) string {
	var text string
	switch e.Kind {
	case EventIssueCreated:
		text = fmt.Sprintf("🆕 %s created %s «%s»", e.Actor, e.IssueKey, e.Title)
	case EventStatusChanged:
		text = fmt.Sprintf("🔄 %s moved %s «%s»: %s → %s", e.Actor, e.IssueKey, e.Title, e.From, e.To)
	case EventSprintStarted:
		text = fmt.Sprintf("🏁 %s started", e.Sprint)
	case EventMeeting:
		text = fmt.Sprintf("📅 %s — %s", e.Title, e.Text)
	case EventAnnouncement:
		text = fmt.Sprintf("📣 %s: %s", e.Actor, e.Title)
		if e.Text != "" {
			text += "\n" + e.Text
		}
	default:
		text = fmt.Sprintf("✅ %s completed", e.Sprint)
	}
	return text + "\n" + link
}
