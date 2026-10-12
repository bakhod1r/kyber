// Package domain is the Audit context: an append-only log of every change made through the
// API — who did what, to which project, when, and with what result.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalidEntry = errors.New("an audit entry needs an actor, an action and a time")

// Entry is one audited action. Action is the API operation ("PUT /api/v1/projects/{key}/workflow"),
// Target the concrete path. Request bodies are never stored (they may hold secrets).
type Entry struct {
	At      time.Time
	Actor   string
	Action  string
	Target  string
	Project string // "" when the action is not tied to one project
	Status  int
}

func NewEntry(actor, action, target, project string, status int, at time.Time) (Entry, error) {
	action = strings.TrimSpace(action)
	if actor == "" || action == "" || at.IsZero() {
		return Entry{}, ErrInvalidEntry
	}
	return Entry{At: at.UTC(), Actor: actor, Action: action, Target: target, Project: project, Status: status}, nil
}

// ProjectOf returns the project of a request from its {key} path value or an issue key ("KYB-12").
func ProjectOf(projectKey, issueKey string) string {
	if projectKey != "" {
		return projectKey
	}
	if i := strings.LastIndexByte(issueKey, '-'); i > 0 {
		return issueKey[:i]
	}
	return ""
}

// Repository is the append-only audit store, scoped to the request's workspace.
type Repository interface {
	Append(ctx context.Context, e Entry) error
	// ForProject returns the project's newest entries first, at most limit.
	ForProject(ctx context.Context, project string, limit int) ([]Entry, error)
}
