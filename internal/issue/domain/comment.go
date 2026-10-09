package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCommentBody = errors.New("comment must be 1-10000 characters")

const maxComment = 10_000

type CommentID string

// Comment is its own aggregate (referencing the issue by ID) so issues never load all comments.
type Comment struct {
	id        CommentID
	issueID   IssueID
	author    UserID
	body      string
	createdAt time.Time
	events    []Event
}

func NewComment(id CommentID, issueID IssueID, issueKey IssueKey, author UserID, body string, at time.Time) (*Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > maxComment {
		return nil, ErrInvalidCommentBody
	}
	c := &Comment{id: id, issueID: issueID, author: author, body: body, createdAt: at}
	c.events = append(c.events, CommentAdded{ID: id, IssueID: issueID, IssueKey: issueKey, Author: author})
	return c, nil
}

// RehydrateComment rebuilds a persisted comment (repository use only).
func RehydrateComment(id CommentID, issueID IssueID, author UserID, body string, at time.Time) *Comment {
	return &Comment{id: id, issueID: issueID, author: author, body: body, createdAt: at}
}

func (c *Comment) ID() CommentID        { return c.id }
func (c *Comment) IssueID() IssueID     { return c.issueID }
func (c *Comment) Author() UserID       { return c.author }
func (c *Comment) Body() string         { return c.body }
func (c *Comment) CreatedAt() time.Time { return c.createdAt }

func (c *Comment) PullEvents() []Event {
	ev := c.events
	c.events = nil
	return ev
}

// CommentRepository is the persistence port for comments; Add writes events to the outbox atomically.
type CommentRepository interface {
	Add(ctx context.Context, c *Comment, events []Event) error
	ListByIssue(ctx context.Context, issueID IssueID) ([]*Comment, error) // oldest first
}
