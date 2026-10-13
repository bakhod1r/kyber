package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidFilter  = errors.New("invalid filter")
	ErrFilterNotFound = errors.New("filter not found")
	ErrFilterConflict = errors.New("filter was changed concurrently")
)

const maxFilterName = 60

type FilterID string

// Filter is a saved JQL query (Jira's saved filter). Its owner edits it; shared filters are
// visible to everyone in the workspace (each viewer still only sees issues they may browse).
type Filter struct {
	ID      FilterID
	Owner   string
	Name    string
	JQL     string
	Shared  bool
	Version int
}

// NewFilter validates the name and the query.
func NewFilter(id FilterID, owner, name, jql string, shared bool) (*Filter, error) {
	f := &Filter{ID: id, Owner: owner}
	return f, f.Edit(name, jql, shared)
}

// Edit replaces the name, query and sharing; on error nothing changes.
func (f *Filter) Edit(name, jql string, shared bool) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxFilterName {
		return fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidFilter, maxFilterName)
	}
	if _, err := Parse(jql); err != nil {
		return err
	}
	f.Name, f.JQL, f.Shared = name, strings.TrimSpace(jql), shared
	return nil
}

// FilterRepository stores filters in the workspace of ctx.
type FilterRepository interface {
	// SaveFilter inserts (Version 0) or updates with optimistic locking, then bumps Version.
	SaveFilter(ctx context.Context, f *Filter) error
	FilterByID(ctx context.Context, id FilterID) (*Filter, error)
	// FiltersFor lists the user's own filters and the workspace's shared ones, by name.
	FiltersFor(ctx context.Context, user string) ([]*Filter, error)
	DeleteFilter(ctx context.Context, id FilterID) error
}
