// Package domain is the Goals context: outcomes a project commits to (Jira's Goals), each with a
// status, a target date and an optional JQL query whose issues measure its progress.
package domain

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidGoal  = errors.New("invalid goal")
	ErrGoalNotFound = errors.New("goal not found")
	ErrGoalConflict = errors.New("goal was changed concurrently")
)

const (
	maxName = 120
	maxJQL  = 2000
)

type GoalID string

// Status is how the goal is going, as its owner reports it.
type Status string

const (
	StatusOnTrack  Status = "on_track"
	StatusAtRisk   Status = "at_risk"
	StatusOffTrack Status = "off_track"
	StatusDone     Status = "done"
)

var statuses = []Status{StatusOnTrack, StatusAtRisk, StatusOffTrack, StatusDone}

// Goal belongs to one project (by key). Its issues are those its JQL finds ("" = none).
type Goal struct {
	ID      GoalID
	Project string
	Owner   string
	Name    string
	Target  string // YYYY-MM-DD, "" = no target date
	JQL     string
	Status  Status
	Version int
}

// NewGoal starts a goal on track.
func NewGoal(id GoalID, project, owner, name, target, jql string) (*Goal, error) {
	g := &Goal{ID: id, Project: project, Owner: owner, Status: StatusOnTrack}
	return g, g.Redefine(name, target, jql)
}

// Redefine replaces the name, target date and query; on error nothing changes.
func (g *Goal) Redefine(name, target, jql string) error {
	name, target, jql = strings.TrimSpace(name), strings.TrimSpace(target), strings.TrimSpace(jql)
	if name == "" || utf8.RuneCountInString(name) > maxName {
		return fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidGoal, maxName)
	}
	if target != "" {
		if _, err := time.Parse(time.DateOnly, target); err != nil {
			return fmt.Errorf("%w: target date must be YYYY-MM-DD", ErrInvalidGoal)
		}
	}
	if utf8.RuneCountInString(jql) > maxJQL {
		return fmt.Errorf("%w: query must be at most %d characters", ErrInvalidGoal, maxJQL)
	}
	g.Name, g.Target, g.JQL = name, target, jql
	return nil
}

// Report records how the goal is going.
func (g *Goal) Report(s Status) error {
	if !slices.Contains(statuses, s) {
		return fmt.Errorf("%w: status must be on_track, at_risk, off_track or done", ErrInvalidGoal)
	}
	g.Status = s
	return nil
}

// Progress counts the goal's issues and how many of them are done.
type Progress struct{ Done, Total int }

// Percent is the rounded share of done issues (0 without issues).
func (p Progress) Percent() int {
	if p.Total == 0 {
		return 0
	}
	return (p.Done*100 + p.Total/2) / p.Total
}

// Repository stores goals in the workspace of ctx.
type Repository interface {
	// Save inserts (Version 0) or updates with optimistic locking, then bumps Version.
	Save(ctx context.Context, g *Goal) error
	ByID(ctx context.Context, id GoalID) (*Goal, error)
	// ForProject lists the project's goals by target date (none last), then name.
	ForProject(ctx context.Context, project string) ([]*Goal, error)
	Delete(ctx context.Context, id GoalID) error
}
