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

// Comment is a remark pinned to one element of a board (S86); its board's conversation members see it.
type Comment struct {
	ID           CommentID
	Workspace    WorkspaceID
	Board        WhiteboardID
	Conversation ConversationID
	Element      ElementID
	Author       UserID
	Body         string
	CreatedAt    time.Time
	events       []CommentAdded
}

var (
	ErrInvalidComment = errors.New("a comment is 1–2000 characters on an element")
	ErrCommentMissing = errors.New("comment not found")
)

// CommentAdded is published (outbox) so that @mentions in it notify people, like a chat message in the board's
// conversation.
type CommentAdded struct {
	Comment      CommentID      `json:"id"`
	Conversation ConversationID `json:"conversation_id"`
	Author       UserID         `json:"author_id"`
	Body         string         `json:"body"`
}

func (CommentAdded) EventName() string { return "whiteboard.comment_added" }

// Comment pins a remark to element el.
func (b *Whiteboard) Comment(id CommentID, el ElementID, by UserID, body string, now time.Time) (*Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 2000 || !elementID.MatchString(string(el)) {
		return nil, ErrInvalidComment
	}
	c := &Comment{ID: id, Workspace: b.Workspace, Board: b.ID, Conversation: b.Conversation, Element: el, Author: by, Body: body, CreatedAt: now.UTC()}
	c.events = append(c.events, CommentAdded{Comment: id, Conversation: b.Conversation, Author: by, Body: body})
	return c, nil
}

func (c *Comment) PullEvents() []CommentAdded {
	ev := c.events
	c.events = nil
	return ev
}

// Comments stores comments.
type Comments interface {
	Add(ctx context.Context, c *Comment) error // with its events (outbox)
	Remove(ctx context.Context, id CommentID) error
	ByID(ctx context.Context, id CommentID) (*Comment, error)        // ErrCommentMissing
	OnBoard(ctx context.Context, b WhiteboardID) ([]*Comment, error) // oldest first
}
