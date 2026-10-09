package domain

// Event is a domain event raised by an aggregate and published via the outbox.
type Event interface{ eventName() string }

type IssueCreated struct {
	ID  IssueID
	Key IssueKey
}

type IssueTransitioned struct {
	ID       IssueID
	From, To StatusID
}

func (IssueCreated) eventName() string      { return "issue.created" }
func (IssueTransitioned) eventName() string { return "issue.transitioned" }
