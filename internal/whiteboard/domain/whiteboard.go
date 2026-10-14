// Package domain is the Whiteboard context: Zoom-style boards drawn together in a conversation (a huddle or
// a channel), e.g. an architecture diagram. A board is a set of elements; every element carries its own
// version, so concurrent edits resolve per element: the first change to a version wins and the loser is
// told the current state (last-writer-wins needs no CRDT for shapes on a board).
package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidTitle      = errors.New("a whiteboard title is 1–120 characters")
	ErrInvalidElement    = errors.New("invalid element")
	ErrElementExists     = errors.New("the element already exists")
	ErrElementNotFound   = errors.New("the element does not exist")
	ErrStale             = errors.New("the element changed meanwhile")
	ErrBoardFull         = errors.New("the whiteboard is full")
	ErrWhiteboardMissing = errors.New("whiteboard not found")
)

const (
	MaxElements = 2000
	MaxText     = 2000
	MaxPoints   = 5000
	maxCoord    = 1e6
)

type (
	WhiteboardID   string
	ElementID      string
	UserID         string
	WorkspaceID    string
	ConversationID string
	ElementKind    string
	OpKind         string
)

const (
	KindRect     ElementKind = "rect"
	KindEllipse  ElementKind = "ellipse"
	KindArrow    ElementKind = "arrow"
	KindLine     ElementKind = "line"
	KindFreehand ElementKind = "freehand"
	KindText     ElementKind = "text"
	KindSticky   ElementKind = "sticky"

	OpAdd    OpKind = "add"
	OpUpdate OpKind = "update"
	OpDelete OpKind = "delete"
)

var kinds = map[ElementKind]bool{KindRect: true, KindEllipse: true, KindArrow: true, KindLine: true, KindFreehand: true, KindText: true, KindSticky: true}

var (
	color     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	elementID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

type Point struct{ X, Y float64 }

// Stroke is a line weight (S91); 0 draws the default.
type Stroke int

const (
	StrokeDefault Stroke = iota
	StrokeThin
	StrokeMedium
	StrokeBold
)

// Element is one shape. W/H size boxes; Points draw lines, arrows and freehand strokes (relative to X/Y).
type Element struct {
	ID         ElementID
	Kind       ElementKind
	X, Y, W, H float64
	Points     []Point
	Text       string
	Color      string
	Stroke     Stroke // S91 styles
	Fill       bool   // shapes filled with their colour (a light tint)
	Dashed     bool
	Z          float64 // S93: layer order, lowest first; ties by last change
	Version    int     // incremented by every applied change
	UpdatedBy  UserID
	UpdatedAt  time.Time
	Deleted    bool // only in an OpResult
}

func coord(v float64) bool { return v > -maxCoord && v < maxCoord }

func (e Element) valid() error {
	bad := func(why string) error { return fmt.Errorf("%w: %s", ErrInvalidElement, why) }
	switch {
	case !elementID.MatchString(string(e.ID)):
		return bad("id")
	case !kinds[e.Kind]:
		return bad("kind")
	case !coord(e.X) || !coord(e.Y) || e.W < 0 || e.H < 0 || e.W > maxCoord || e.H > maxCoord:
		return bad("position or size")
	case len(e.Points) > MaxPoints:
		return bad("too many points")
	case utf8.RuneCountInString(e.Text) > MaxText:
		return bad("text too long")
	case e.Color != "" && !color.MatchString(e.Color):
		return bad("color")
	case e.Stroke < StrokeDefault || e.Stroke > StrokeBold:
		return bad("stroke")
	case !coord(e.Z):
		return bad("order")
	}
	for _, p := range e.Points {
		if !coord(p.X) || !coord(p.Y) {
			return bad("point")
		}
	}
	return nil
}

type Whiteboard struct {
	ID           WhiteboardID
	Workspace    WorkspaceID
	Conversation ConversationID
	Title        string
	CreatedBy    UserID
	CreatedAt    time.Time
	Elements     map[ElementID]Element
}

func New(id WhiteboardID, ws WorkspaceID, conv ConversationID, title string, by UserID, now time.Time) (*Whiteboard, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		return nil, ErrInvalidTitle
	}
	return &Whiteboard{ID: id, Workspace: ws, Conversation: conv, Title: title, CreatedBy: by, CreatedAt: now.UTC(), Elements: map[ElementID]Element{}}, nil
}

// Op is one change. For update and delete, Element.Version is the version the change was made on.
type Op struct {
	Kind    OpKind
	Element Element
}

// OpResult reports whether an op applied; either way Element is the element's state now (Deleted when gone).
type OpResult struct {
	Applied bool
	Error   error
	Element Element
}

// Apply runs ops in order; each succeeds or fails on its own.
func (b *Whiteboard) Apply(by UserID, ops []Op, now time.Time) []OpResult {
	out := make([]OpResult, len(ops))
	for i, op := range ops {
		out[i] = b.apply(by, op, now.UTC())
	}
	return out
}

func (b *Whiteboard) apply(by UserID, op Op, now time.Time) OpResult {
	e := op.Element
	cur, exists := b.Elements[e.ID]
	fail := func(err error) OpResult {
		if exists {
			return OpResult{Error: err, Element: cur}
		}
		return OpResult{Error: err, Element: Element{ID: e.ID, Deleted: true}}
	}
	switch op.Kind {
	case OpAdd:
		if exists {
			return fail(ErrElementExists)
		}
		if err := e.valid(); err != nil {
			return OpResult{Error: err}
		}
		if len(b.Elements) >= MaxElements {
			return OpResult{Error: ErrBoardFull}
		}
		e.Version, e.UpdatedBy, e.UpdatedAt, e.Deleted = 1, by, now, false
	case OpUpdate:
		if !exists {
			return fail(ErrElementNotFound)
		}
		if e.Version != cur.Version {
			return fail(ErrStale)
		}
		if err := e.valid(); err != nil {
			return fail(err)
		}
		e.Version, e.UpdatedBy, e.UpdatedAt, e.Deleted = cur.Version+1, by, now, false
	case OpDelete:
		if !exists {
			return fail(ErrElementNotFound)
		}
		if e.Version != cur.Version {
			return fail(ErrStale)
		}
		delete(b.Elements, e.ID)
		return OpResult{Applied: true, Element: Element{ID: e.ID, Version: cur.Version + 1, UpdatedBy: by, UpdatedAt: now, Deleted: true}}
	default:
		return OpResult{Error: fmt.Errorf("%w: op", ErrInvalidElement)}
	}
	b.Elements[e.ID] = e
	return OpResult{Applied: true, Element: e}
}

// Repository stores whiteboards. Save writes the elements changed by applied results.
type Repository interface {
	Create(ctx context.Context, b *Whiteboard) error
	ByID(ctx context.Context, id WhiteboardID) (*Whiteboard, error) // ErrWhiteboardMissing
	InConversation(ctx context.Context, conv ConversationID) ([]*Whiteboard, error)
	// Save persists applied results atomically, but only if each element is still at the version the results
	// were based on (else ErrStale: reload and apply again).
	Save(ctx context.Context, id WhiteboardID, applied []OpResult) error
}
