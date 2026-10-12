package domain

import (
	"errors"
	"fmt"
)

var ErrInvalidParent = errors.New("invalid parent: epics hold stories, tasks and bugs; those hold sub-tasks; same project")

// Parent returns the parent issue's id ("" when none).
func (i *Issue) Parent() IssueID     { return i.parent }
func (i *Issue) ParentKey() IssueKey { return i.parentKey }

// CheckParent validates putting an issue of type child under parent (nil = no parent) in project,
// before the child exists — so a rejected create does not burn an issue number.
func CheckParent(project string, child IssueType, parent *Issue) error {
	switch {
	case parent == nil && child == TypeSubtask:
		return fmt.Errorf("a sub-task needs a parent: %w", ErrInvalidParent)
	case parent != nil && (parent.key.Project() != project || !canParent(parent.typ, child)):
		return fmt.Errorf("%s under %s (%s): %w", child, parent.key, parent.typ, ErrInvalidParent)
	}
	return nil
}

// canParent tells whether an issue of type parent may hold one of type child (Jira's hierarchy).
func canParent(parent, child IssueType) bool {
	switch parent {
	case TypeEpic:
		return child == TypeStory || child == TypeTask || child == TypeBug
	case TypeStory, TypeTask, TypeBug:
		return child == TypeSubtask
	}
	return false
}

// SetParent puts the issue under p (nil removes the parent; sub-tasks always need one).
func (i *Issue) SetParent(p *Issue) error {
	if p == nil {
		if i.typ == TypeSubtask {
			return fmt.Errorf("a sub-task needs a parent: %w", ErrInvalidParent)
		}
		i.parent, i.parentKey = "", IssueKey{}
		return nil
	}
	if p.id == i.id || p.key.Project() != i.key.Project() || !canParent(p.typ, i.typ) {
		return fmt.Errorf("%s (%s) under %s (%s): %w", i.key, i.typ, p.key, p.typ, ErrInvalidParent)
	}
	i.parent, i.parentKey = p.id, p.key
	return nil
}
