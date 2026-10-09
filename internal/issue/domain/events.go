package domain

// Event is a domain event raised by an aggregate and published via the outbox.
type Event interface{ EventName() string }

type IssueCreated struct {
	ID  IssueID
	Key IssueKey
}

type IssueTransitioned struct {
	ID       IssueID
	From, To StatusID
}

func (IssueCreated) EventName() string      { return "issue.created" }
func (IssueTransitioned) EventName() string { return "issue.transitioned" }
