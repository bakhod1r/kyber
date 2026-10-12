package domain

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrInvalidLink  = errors.New("invalid link: choose blocks, relates, duplicates or clones, between two different issues")
	ErrLinkExists   = errors.New("these issues are already linked this way")
	ErrLinkNotFound = errors.New("link not found")
)

type (
	LinkID   string
	LinkType string
)

const (
	LinkBlocks     LinkType = "blocks"
	LinkRelates    LinkType = "relates"
	LinkDuplicates LinkType = "duplicates"
	LinkClones     LinkType = "clones"
)

// phrases reads a link from its source (outward) and from its target (inward), as Jira does.
var phrases = map[LinkType][2]string{
	LinkBlocks:     {"blocks", "is blocked by"},
	LinkRelates:    {"relates to", "relates to"},
	LinkDuplicates: {"duplicates", "is duplicated by"},
	LinkClones:     {"clones", "is cloned by"},
}

// LinkTypes lists the supported link types.
func LinkTypes() []LinkType { return []LinkType{LinkBlocks, LinkRelates, LinkDuplicates, LinkClones} }

// Link connects two issues: From <type> To (e.g. KYB-1 blocks KYB-2).
type Link struct {
	ID             LinkID
	Type           LinkType
	From, To       IssueID
	FromKey, ToKey IssueKey
	linkedBy       IssueLinked
}

func NewLink(id LinkID, typ LinkType, from, to *Issue, by UserID) (Link, error) {
	if _, ok := phrases[typ]; !ok || from.id == to.id {
		return Link{}, fmt.Errorf("%s %s %s: %w", from.key, typ, to.key, ErrInvalidLink)
	}
	l := Link{ID: id, Type: typ, From: from.id, To: to.id, FromKey: from.key, ToKey: to.key}
	l.linkedBy = l.event(by)
	return l, nil
}

// PhraseFrom reads the link from issue's side ("blocks" or "is blocked by").
func (l Link) PhraseFrom(issue IssueID) string {
	if issue == l.From {
		return phrases[l.Type][0]
	}
	return phrases[l.Type][1]
}

// Other returns the issue at the other end.
func (l Link) Other(issue IssueID) IssueID {
	if issue == l.From {
		return l.To
	}
	return l.From
}

func (l Link) OtherKey(issue IssueID) IssueKey {
	if issue == l.From {
		return l.ToKey
	}
	return l.FromKey
}

// SameAs tells whether two links say the same thing ("relates to" has no direction).
func (l Link) SameAs(o Link) bool {
	if l.Type != o.Type {
		return false
	}
	if l.From == o.From && l.To == o.To {
		return true
	}
	return l.Type == LinkRelates && l.From == o.To && l.To == o.From
}

// IssueLinked / IssueUnlinked are recorded in both issues' history.
type IssueLinked struct {
	ID      LinkID   `json:"id"`
	Type    LinkType `json:"type"`
	From    IssueID  `json:"from"`
	FromKey IssueKey `json:"from_key"`
	To      IssueID  `json:"to"`
	ToKey   IssueKey `json:"to_key"`
	By      UserID   `json:"by"`
}

type IssueUnlinked IssueLinked

func (IssueLinked) EventName() string   { return "issue.linked" }
func (IssueUnlinked) EventName() string { return "issue.unlinked" }

func (l Link) event(by UserID) IssueLinked {
	return IssueLinked{ID: l.ID, Type: l.Type, From: l.From, FromKey: l.FromKey, To: l.To, ToKey: l.ToKey, By: by}
}

// Linked is the event recording who created the link.
func (l Link) Linked() IssueLinked { return l.linkedBy }

func (l Link) Unlinked(by UserID) IssueUnlinked { return IssueUnlinked(l.event(by)) }

// LinkRepository stores links, scoped to the request's workspace.
type LinkRepository interface {
	// Add stores the link and its event atomically; ErrLinkExists when an equal link exists.
	Add(ctx context.Context, l Link, events []Event) error
	// Remove deletes the link and stores its event atomically; ErrLinkNotFound when missing.
	Remove(ctx context.Context, id LinkID, events []Event) error
	ByID(ctx context.Context, id LinkID) (Link, error)
	// ForIssue returns the links touching the issue, oldest first.
	ForIssue(ctx context.Context, issue IssueID) ([]Link, error)
}
