package domain

// Event is a domain event raised by an aggregate and stored in the outbox.
// JSON field names are a published contract; change them only additively.
type Event interface{ EventName() string }

type IssueCreated struct {
	ID       IssueID   `json:"id"`
	Key      IssueKey  `json:"key"`
	Type     IssueType `json:"type"`
	Title    string    `json:"title"`
	Reporter UserID    `json:"reporter"`
}

type IssueTransitioned struct {
	ID   IssueID  `json:"id"`
	Key  IssueKey `json:"key"`
	From StatusID `json:"from"`
	To   StatusID `json:"to"`
}

func (IssueCreated) EventName() string      { return "issue.created" }
func (IssueTransitioned) EventName() string { return "issue.transitioned" }

type IssueEdited struct {
	ID     IssueID  `json:"id"`
	Key    IssueKey `json:"key"`
	Fields []string `json:"fields"`
}

type IssueAssigned struct {
	ID   IssueID  `json:"id"`
	Key  IssueKey `json:"key"`
	From UserID   `json:"from"`
	To   UserID   `json:"to"`
	By   UserID   `json:"by"`
}

type CommentAdded struct {
	ID       CommentID `json:"id"`
	IssueID  IssueID   `json:"issue_id"`
	IssueKey IssueKey  `json:"issue_key"`
	Author   UserID    `json:"author"`
	Body     string    `json:"body"`
}

func (IssueEdited) EventName() string   { return "issue.edited" }
func (IssueAssigned) EventName() string { return "issue.assigned" }
func (CommentAdded) EventName() string  { return "comment.added" }

type IssueSprintChanged struct {
	ID   IssueID  `json:"id"`
	Key  IssueKey `json:"key"`
	From SprintID `json:"from"`
	To   SprintID `json:"to"`
}

func (IssueSprintChanged) EventName() string { return "issue.sprint_changed" }
