package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// CommentID identifies a comment.
type CommentID string

// Comment is a remark on one paragraph of a page (S86): Paragraph is its index among the page's paragraphs
// (blocks split by blank lines) and Quote the start of the text it was made on, so it can still be placed
// after the page changes. Whoever may read the page sees it.
type Comment struct {
	ID        CommentID
	Page      PageID
	Project   string
	Paragraph int
	Quote     string
	Author    string
	Body      string
	CreatedAt time.Time
	events    []CommentAdded
}

var (
	ErrInvalidComment = errors.New("a comment is 1–2000 characters on a paragraph")
	ErrCommentMissing = errors.New("comment not found")
)

// CommentAdded is published (outbox) so that people @mentioned in it are told.
type CommentAdded struct {
	Comment   CommentID `json:"id"`
	Workspace string    `json:"workspace"`
	Project   string    `json:"project"`
	Page      PageID    `json:"page_id"`
	Title     string    `json:"title"`
	Author    string    `json:"author_id"`
	Body      string    `json:"body"`
}

func (CommentAdded) EventName() string { return "doc.comment_added" }

// Comment pins a remark to paragraph para of the page, in workspace ws.
func (p *Page) Comment(id CommentID, ws string, para int, quote, by, body string, now time.Time) (*Comment, error) {
	body, quote = strings.TrimSpace(body), strings.TrimSpace(quote)
	if para < 0 || body == "" || utf8.RuneCountInString(body) > 2000 || utf8.RuneCountInString(quote) > 200 {
		return nil, ErrInvalidComment
	}
	c := &Comment{ID: id, Page: p.ID, Project: p.Project, Paragraph: para, Quote: quote, Author: by, Body: body, CreatedAt: now.UTC()}
	c.events = append(c.events, CommentAdded{Comment: id, Workspace: ws, Project: p.Project, Page: p.ID, Title: p.Title, Author: by, Body: body})
	return c, nil
}

func (c *Comment) PullEvents() []CommentAdded {
	ev := c.events
	c.events = nil
	return ev
}

// Comments stores comments in the workspace of ctx.
type Comments interface {
	Add(ctx context.Context, c *Comment) error // with its events (outbox)
	Remove(ctx context.Context, id CommentID) error
	ByID(ctx context.Context, id CommentID) (*Comment, error) // ErrCommentMissing
	OnPage(ctx context.Context, p PageID) ([]*Comment, error) // oldest first
}
