// Package domain is the Notification context: who must be told about what.
package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var ErrNotificationNotFound = errors.New("notification not found")

type (
	UserID         string
	NotificationID string
	Kind           string
)

const (
	KindAssigned  Kind = "assigned"
	KindCommented Kind = "commented"
	KindMentioned Kind = "mentioned"
)

// Notification is one message to one recipient about one event. SourceEvent (the
// outbox message id) plus Recipient is unique, which makes delivery idempotent.
type Notification struct {
	ID          NotificationID
	Recipient   UserID
	Workspace   string // the issue's workspace: inboxes are per workspace (ADR-0004)
	Kind        Kind
	IssueKey    string
	IssueTitle  string
	ActorName   string
	Excerpt     string
	SourceEvent int64
	CreatedAt   time.Time
	ReadAt      time.Time // zero = unread
}

func (n Notification) Read() bool { return !n.ReadAt.IsZero() }

// AssignmentRecipients: the new assignee, unless they assigned themselves.
func AssignmentRecipients(to, by UserID) []UserID {
	if to == "" || to == by {
		return nil
	}
	return []UserID{to}
}

// CommentRecipients: reporter and assignee are told about the comment, mentioned
// users that they were mentioned (which wins); the author is never notified.
// Membership of mentioned users is checked by the application layer.
func CommentRecipients(author, reporter, assignee UserID, mentioned []UserID) map[UserID]Kind {
	out := map[UserID]Kind{}
	for _, u := range []UserID{reporter, assignee} {
		if u != "" {
			out[u] = KindCommented
		}
	}
	for _, u := range mentioned {
		if u != "" {
			out[u] = KindMentioned
		}
	}
	delete(out, author)
	return out
}

var mentionRe = regexp.MustCompile(`(?:^|[\s(\[{,;])@([^\s@()\[\]{}<>,;]+@[^\s@()\[\]{}<>,;]+\.[^\s@()\[\]{}<>,;]+)`)

// ParseMentions returns the distinct, lower-cased emails written as "@email" in body.
func ParseMentions(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mentionRe.FindAllStringSubmatch(body, -1) {
		email := strings.ToLower(strings.TrimRight(m[1], ".!?:'\""))
		if !seen[email] {
			seen[email] = true
			out = append(out, email)
		}
	}
	return out
}

// Inbox is one user's notifications within one workspace.
type Inbox struct {
	Workspace string
	User      UserID
}

func (in Inbox) holds(n Notification) bool {
	return n.Recipient == in.User && n.Workspace == in.Workspace
}

// Holds reports whether n belongs to the inbox (for in-memory adapters).
func (in Inbox) Holds(n Notification) bool { return in.holds(n) }

// Repository is the persistence port for notifications.
type Repository interface {
	// Add stores n unless (SourceEvent, Recipient) already exists (idempotent delivery).
	Add(ctx context.Context, n Notification) error
	// ListFor returns the user's notifications, newest first.
	ListFor(ctx context.Context, in Inbox, unreadOnly bool, limit int) ([]Notification, error)
	UnreadCount(ctx context.Context, in Inbox) (int, error)
	// MarkRead fails with ErrNotificationNotFound unless id belongs to the inbox.
	MarkRead(ctx context.Context, in Inbox, id NotificationID, at time.Time) error
	MarkAllRead(ctx context.Context, in Inbox, at time.Time) error
}
