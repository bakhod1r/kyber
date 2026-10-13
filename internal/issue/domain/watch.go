package domain

import "context"

// WatcherRepository stores who watches which issue (Jira's watchers): they hear about the
// issue's comments and status changes. Watching is idempotent.
type WatcherRepository interface {
	Watch(ctx context.Context, issue IssueID, user UserID) error
	Unwatch(ctx context.Context, issue IssueID, user UserID) error
	// Watchers lists the issue's watchers in the order they started watching.
	Watchers(ctx context.Context, issue IssueID) ([]UserID, error)
}
