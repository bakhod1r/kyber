package domain

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// BookmarkID identifies a bookmark.
type BookmarkID string

// BookmarkKind: a doc (Docs context, by id) or a web link.
type BookmarkKind string

const (
	BookmarkDoc  BookmarkKind = "doc"
	BookmarkLink BookmarkKind = "link"
)

// Bookmark is something kept at hand in a conversation's header (S79), like Slack's canvases and bookmarks.
type Bookmark struct {
	ID           BookmarkID
	Workspace    WorkspaceID
	Conversation ConversationID
	Kind         BookmarkKind
	Ref          string // the doc id, or the http(s) URL
	Title        string
	AddedBy      UserID
	AddedAt      time.Time
}

var (
	ErrInvalidBookmark  = errors.New("a bookmark is a doc or an http(s) link with a title of 1–100 characters")
	ErrBookmarkNotFound = errors.New("bookmark not found")
	ErrTooManyBookmarks = errors.New("a conversation keeps at most 20 bookmarks")
)

// MaxBookmarks bounds a conversation's bookmarks.
const MaxBookmarks = 20

func NewBookmark(id BookmarkID, ws WorkspaceID, conv ConversationID, kind BookmarkKind, ref, title string, by UserID, now time.Time) (*Bookmark, error) {
	ref, title = strings.TrimSpace(ref), strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n == 0 || n > 100 || ref == "" || len(ref) > 2000 {
		return nil, ErrInvalidBookmark
	}
	switch kind {
	case BookmarkDoc:
		if strings.ContainsAny(ref, "/?#: ") {
			return nil, ErrInvalidBookmark
		}
	case BookmarkLink:
		u, err := url.Parse(ref)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return nil, ErrInvalidBookmark
		}
	default:
		return nil, ErrInvalidBookmark
	}
	return &Bookmark{ID: id, Workspace: ws, Conversation: conv, Kind: kind, Ref: ref, Title: title, AddedBy: by, AddedAt: now.UTC()}, nil
}

// Bookmarks stores bookmarks.
type Bookmarks interface {
	Add(ctx context.Context, b *Bookmark) error
	Remove(ctx context.Context, id BookmarkID) error
	ByID(ctx context.Context, id BookmarkID) (*Bookmark, error) // ErrBookmarkNotFound
	// In lists a conversation's bookmarks in the order they were added.
	In(ctx context.Context, conv ConversationID) ([]*Bookmark, error)
}
