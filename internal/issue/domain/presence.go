package domain

import (
	"context"
	"time"
)

// PresenceRegistry remembers who is looking at which issue right now. Entries expire on their
// own; it is ephemeral state (each replica keeps its own, see ADR-0013).
type PresenceRegistry interface {
	// Seen records at now that user views issue until the given time; arrived is true when the
	// user was not present before (so others should be told).
	Seen(ctx context.Context, issue IssueID, user UserID, now, until time.Time) (arrived bool, err error)
	// Present lists who views the issue at now, in order of arrival.
	Present(ctx context.Context, issue IssueID, now time.Time) ([]UserID, error)
}
