package domain

import "context"

// Repository is the persistence port for the Issue aggregate.
type Repository interface {
	// Save inserts (Version 0) or updates (Version must match) the issue and appends
	// events to the outbox in the same transaction. Mismatch → ErrConcurrentModification.
	Save(ctx context.Context, is *Issue, events []Event) error
	ByKey(ctx context.Context, key IssueKey) (*Issue, error) // ErrIssueNotFound
	ListByProject(ctx context.Context, project string, status *StatusID) ([]*Issue, error)
}

// KeyAllocator hands out the next sequential key of a project (ACL to the Project context).
type KeyAllocator interface {
	Next(ctx context.Context, project string) (IssueKey, error)
}
