package domain

import "context"

// Repository is the persistence port for the Issue aggregate.
type Repository interface {
	// Save inserts (Version 0) or updates (Version must match) the issue and appends
	// events to the outbox in the same transaction. Mismatch → ErrConcurrentModification.
	Save(ctx context.Context, is *Issue, events []Event) error
	ByKey(ctx context.Context, key IssueKey) (*Issue, error) // ErrIssueNotFound
	// ListByProject returns issues ordered by rank (then number).
	ListByProject(ctx context.Context, project string, f ListFilter) ([]*Issue, error)
	// LastRank is the highest rank in the project ("" when it has no issues).
	LastRank(ctx context.Context, project string) (Rank, error)
	// NextRank / PrevRank return the neighbouring rank after / before r ("" at the ends).
	NextRank(ctx context.Context, project string, r Rank) (Rank, error)
	PrevRank(ctx context.Context, project string, r Rank) (Rank, error)
}

// ListFilter narrows a project listing; nil fields do not filter.
// A Sprint pointing at "" selects the backlog (issues in no sprint).
type ListFilter struct {
	Status *StatusID
	Sprint *SprintID
}

// KeyAllocator hands out the next sequential key of a project (ACL to the Project context).
type KeyAllocator interface {
	Next(ctx context.Context, project string) (IssueKey, error)
}
