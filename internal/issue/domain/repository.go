package domain

import "context"

// Repository is the persistence port for the Issue aggregate.
type Repository interface {
	Save(ctx context.Context, is *Issue) error
	ByKey(ctx context.Context, key IssueKey) (*Issue, error) // ErrIssueNotFound
	ListByProject(ctx context.Context, project string, status *StatusID) ([]*Issue, error)
}

// KeyAllocator hands out the next sequential key of a project (ACL to the Project context).
type KeyAllocator interface {
	Next(ctx context.Context, project string) (IssueKey, error)
}

// EventPublisher delivers domain events (outbox in production).
type EventPublisher interface {
	Publish(ctx context.Context, events ...Event) error
}
