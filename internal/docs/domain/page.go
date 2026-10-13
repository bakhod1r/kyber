// Package domain is the Docs context: a project's Markdown pages (Jira's/Confluence's project
// pages), each save kept as a numbered revision.
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
	ErrInvalidPage  = errors.New("invalid page")
	ErrPageNotFound = errors.New("page not found")
	ErrPageConflict = errors.New("page was changed concurrently")
)

const (
	maxTitle = 200
	maxBody  = 100_000
)

type PageID string

// Page is the current state of a project page. Version is the optimistic lock and equals the
// number of its latest revision.
type Page struct {
	ID        PageID
	Project   string
	Title     string
	Body      string // Markdown
	Author    string
	UpdatedBy string
	UpdatedAt time.Time
	Version   int
}

// Revision is one saved state of a page (its history).
type Revision struct {
	Page   PageID
	Number int
	Title  string
	Body   string
	By     string
	At     time.Time
}

func normalize(title, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitle || strings.ContainsAny(title, "\r\n") {
		return "", "", fmt.Errorf("%w: title must be one line of 1-%d characters", ErrInvalidPage, maxTitle)
	}
	if utf8.RuneCountInString(body) > maxBody {
		return "", "", fmt.Errorf("%w: body must be at most %d characters", ErrInvalidPage, maxBody)
	}
	return title, body, nil
}

// NewPage writes a page and its first revision.
func NewPage(id PageID, project, title, body, by string, at time.Time) (*Page, Revision, error) {
	title, body, err := normalize(title, body)
	if err != nil {
		return nil, Revision{}, err
	}
	p := &Page{ID: id, Project: project, Title: title, Body: body, Author: by, UpdatedBy: by, UpdatedAt: at}
	return p, Revision{Page: id, Number: 1, Title: title, Body: body, By: by, At: at}, nil
}

// Revise replaces the title and body. Saving the same content records nothing (changed = false);
// on error nothing changes.
func (p *Page) Revise(title, body, by string, at time.Time) (rev Revision, changed bool, err error) {
	title, body, err = normalize(title, body)
	if err != nil || (title == p.Title && body == p.Body) {
		return Revision{}, false, err
	}
	p.Title, p.Body, p.UpdatedBy, p.UpdatedAt = title, body, by, at
	return Revision{Page: p.ID, Number: p.Version + 1, Title: title, Body: body, By: by, At: at}, true, nil
}

// Repository stores pages and their history in the workspace of ctx.
type Repository interface {
	// Save stores the page with its new revision atomically: insert when Version is 0, otherwise
	// update the page loaded at Version. Version becomes the revision's number.
	Save(ctx context.Context, p *Page, rev Revision) error
	ByID(ctx context.Context, id PageID) (*Page, error)
	// ForProject lists the project's pages by title.
	ForProject(ctx context.Context, project string) ([]*Page, error)
	// History lists a page's revisions, newest first.
	History(ctx context.Context, id PageID) ([]Revision, error)
	Delete(ctx context.Context, id PageID) error
}
