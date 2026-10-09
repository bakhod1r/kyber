package domain

// Event is a domain event raised by an aggregate and stored in the outbox.
// JSON field names are a published contract; change them only additively.
type Event interface{ EventName() string }

type IssueCreated struct {
	ID    IssueID   `json:"id"`
	Key   IssueKey  `json:"key"`
	Type  IssueType `json:"type"`
	Title string    `json:"title"`
}

type IssueTransitioned struct {
	ID   IssueID  `json:"id"`
	Key  IssueKey `json:"key"`
	From StatusID `json:"from"`
	To   StatusID `json:"to"`
}

func (IssueCreated) EventName() string      { return "issue.created" }
func (IssueTransitioned) EventName() string { return "issue.transitioned" }
