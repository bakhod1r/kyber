// Package domain is the Announcements context: news a project's or a workspace's admins publish
// to everyone in it. Publishing raises an event; the Notification context delivers it.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidAnnouncement  = errors.New("invalid announcement")
	ErrAnnouncementNotFound = errors.New("announcement not found")
)

const (
	maxTitle   = 120
	maxBody    = 10_000
	excerptLen = 280
)

type AnnouncementID string

// Announcement is published once and then read; Project "" addresses the whole workspace.
type Announcement struct {
	ID        AnnouncementID
	Workspace string
	Project   string
	Title     string
	Body      string // Markdown
	Author    string
	At        time.Time
	events    []Event
}

// Event is a domain event written to the outbox.
type Event interface{ EventName() string }

// Published tells the Notification context whom to inform: the project's members, or the
// workspace's when Project is "".
type Published struct {
	ID        AnnouncementID `json:"id"`
	Workspace string         `json:"workspace"`
	Project   string         `json:"project,omitempty"`
	Title     string         `json:"title"`
	Excerpt   string         `json:"excerpt"`
	Author    string         `json:"author"`
}

func (Published) EventName() string { return "announcement.published" }

// Publish writes an announcement and raises Published.
func Publish(id AnnouncementID, workspace, project, title, body, author string, at time.Time) (*Announcement, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitle || strings.ContainsAny(title, "\r\n") {
		return nil, fmt.Errorf("%w: title must be one line of 1-%d characters", ErrInvalidAnnouncement, maxTitle)
	}
	if utf8.RuneCountInString(body) > maxBody {
		return nil, fmt.Errorf("%w: text must be at most %d characters", ErrInvalidAnnouncement, maxBody)
	}
	a := &Announcement{ID: id, Workspace: workspace, Project: project, Title: title, Body: body, Author: author, At: at}
	a.events = append(a.events, Published{ID: id, Workspace: workspace, Project: project, Title: title, Excerpt: excerpt(body), Author: author})
	return a, nil
}

func excerpt(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= excerptLen {
		return string(r)
	}
	return string(r[:excerptLen-1]) + "…"
}

// WorkspaceWide reports whether the announcement addresses the whole workspace.
func (a *Announcement) WorkspaceWide() bool { return a.Project == "" }

// PullEvents returns and clears the raised events.
func (a *Announcement) PullEvents() []Event {
	e := a.events
	a.events = nil
	return e
}

// Repository stores announcements in the workspace of ctx; Save writes the events to the outbox
// in the same transaction.
type Repository interface {
	Save(ctx context.Context, a *Announcement) error
	ByID(ctx context.Context, id AnnouncementID) (*Announcement, error)
	// For lists the newest announcements of a project ("" = the workspace-wide ones).
	For(ctx context.Context, project string, limit int) ([]*Announcement, error)
}
