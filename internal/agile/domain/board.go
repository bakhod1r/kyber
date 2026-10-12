package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidBoard  = errors.New("invalid board")
	ErrBoardNotFound = errors.New("board not found")
	ErrBoardConflict = errors.New("board was changed concurrently")
)

type (
	BoardID   string
	BoardKind string
	WIP       string
)

const (
	KindScrum  BoardKind = "scrum"
	KindKanban BoardKind = "kanban"

	WIPOK    WIP = "ok"
	WIPUnder WIP = "under"
	WIPOver  WIP = "over"
)

const maxBoardName = 60

// Column is a board column showing the issues in its workflow statuses, with optional
// work-in-progress limits (0 = no limit).
type Column struct {
	Name     string   `json:"name"`
	Statuses []string `json:"statuses"`
	Min      int      `json:"min"`
	Max      int      `json:"max"`
}

// WIP tells whether n issues respect the column's limits.
func (c Column) WIP(n int) WIP {
	switch {
	case c.Min > 0 && n < c.Min:
		return WIPUnder
	case c.Max > 0 && n > c.Max:
		return WIPOver
	}
	return WIPOK
}

// Board is a view of a project's issues (Jira's Scrum or Kanban board): columns map
// workflow statuses; a status belongs to at most one column, unmapped ones are hidden.
type Board struct {
	id      BoardID
	project string
	name    string
	kind    BoardKind
	columns []Column
	version int
	follows bool // tracks the workflow until configured by hand
}

// StatusRef is a workflow status as the board sees it.
type StatusRef struct{ ID, Name string }

func NewBoard(id BoardID, project, name string, kind BoardKind, columns []Column) (*Board, error) {
	b := &Board{id: id, project: project}
	if err := b.Configure(name, kind, columns); err != nil {
		return nil, err
	}
	return b, nil
}

// RehydrateBoard rebuilds a stored board without validation.
func RehydrateBoard(id BoardID, project, name string, kind BoardKind, columns []Column, version int) *Board {
	return &Board{id: id, project: project, name: name, kind: kind, columns: copyColumns(columns), version: version}
}

// Configure replaces the board's name, kind and columns; on error nothing changes.
func (b *Board) Configure(name string, kind BoardKind, columns []Column) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxBoardName {
		return fmt.Errorf("name must be 1-%d characters: %w", maxBoardName, ErrInvalidBoard)
	}
	if kind != KindScrum && kind != KindKanban {
		return fmt.Errorf("kind %q: %w", kind, ErrInvalidBoard)
	}
	if len(columns) == 0 {
		return fmt.Errorf("a board needs a column: %w", ErrInvalidBoard)
	}
	names, statuses := map[string]bool{}, map[string]bool{}
	out := copyColumns(columns)
	for i := range out {
		c := &out[i]
		c.Name = strings.TrimSpace(c.Name)
		switch {
		case c.Name == "" || utf8.RuneCountInString(c.Name) > maxBoardName:
			return fmt.Errorf("column %d name: %w", i+1, ErrInvalidBoard)
		case names[strings.ToLower(c.Name)]:
			return fmt.Errorf("duplicate column %q: %w", c.Name, ErrInvalidBoard)
		case c.Min < 0 || c.Max < 0 || (c.Max > 0 && c.Min > c.Max):
			return fmt.Errorf("column %q limits: %w", c.Name, ErrInvalidBoard)
		}
		names[strings.ToLower(c.Name)] = true
		for _, s := range c.Statuses {
			if s == "" || statuses[s] {
				return fmt.Errorf("status %q must be in at most one column: %w", s, ErrInvalidBoard)
			}
			statuses[s] = true
		}
	}
	b.name, b.kind, b.columns, b.follows = name, kind, out, false
	return nil
}

func (b *Board) ID() BoardID       { return b.id }
func (b *Board) Project() string   { return b.project }
func (b *Board) Name() string      { return b.name }
func (b *Board) Kind() BoardKind   { return b.kind }
func (b *Board) Version() int      { return b.version }
func (b *Board) Columns() []Column { return copyColumns(b.columns) }

// ColumnOf returns the index of the column showing status, or -1 when it is unmapped.
func (b *Board) ColumnOf(status string) int {
	for i, c := range b.columns {
		for _, s := range c.Statuses {
			if s == status {
				return i
			}
		}
	}
	return -1
}

func copyColumns(in []Column) []Column {
	out := make([]Column, len(in))
	for i, c := range in {
		c.Statuses = append([]string{}, c.Statuses...)
		out[i] = c
	}
	return out
}

// BoardRepository stores boards, scoped to the request's workspace.
type BoardRepository interface {
	// SaveBoard inserts (version 0) or updates (version must match, else ErrBoardConflict).
	SaveBoard(ctx context.Context, b *Board) error
	BoardByID(ctx context.Context, id BoardID) (*Board, error) // ErrBoardNotFound
	// BoardsByProject returns the project's boards ordered by name, then id.
	BoardsByProject(ctx context.Context, project string) ([]*Board, error)
	DeleteBoard(ctx context.Context, id BoardID) error // ErrBoardNotFound
}

// MarkPersisted is called by repositories after a successful save.
func (b *Board) MarkPersisted() { b.version++ }

// FollowWorkflow marks the board as the project's default board, kept in step with the workflow.
func (b *Board) FollowWorkflow()       { b.follows = true }
func (b *Board) FollowsWorkflow() bool { return b.follows }

// SyncWith rebuilds a following board's columns in workflow order: mapped columns keep their
// name and limits, new statuses get their own column, removed statuses drop out.
func (b *Board) SyncWith(statuses []StatusRef) {
	if !b.follows {
		return
	}
	var out []Column
	used := map[int]bool{}
	for _, st := range statuses {
		i := b.ColumnOf(st.ID)
		switch {
		case i < 0:
			out = append(out, Column{Name: st.Name, Statuses: []string{st.ID}})
		case !used[i]:
			used[i] = true
			c := b.columns[i]
			keep := []string{}
			for _, id := range c.Statuses {
				for _, w := range statuses {
					if w.ID == id {
						keep = append(keep, id)
					}
				}
			}
			c.Statuses = keep
			out = append(out, c)
		}
	}
	if len(out) > 0 {
		b.columns = out
	}
}
