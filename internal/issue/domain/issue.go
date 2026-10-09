package domain

import (
	"errors"
	"strings"
)

var ErrEmptyTitle = errors.New("issue title must not be empty")

type IssueID string

type IssueType string

const (
	TypeEpic    IssueType = "epic"
	TypeStory   IssueType = "story"
	TypeTask    IssueType = "task"
	TypeBug     IssueType = "bug"
	TypeSubtask IssueType = "subtask"
)

// Issue is the aggregate root of the Issue Tracking context.
type Issue struct {
	id     IssueID
	key    IssueKey
	title  string
	typ    IssueType
	status StatusID
	events []Event
}

func NewIssue(id IssueID, key IssueKey, title string, typ IssueType, wf *Workflow) (*Issue, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrEmptyTitle
	}
	is := &Issue{id: id, key: key, title: title, typ: typ, status: wf.Initial()}
	is.record(IssueCreated{ID: id, Key: key})
	return is, nil
}

func (i *Issue) ID() IssueID      { return i.id }
func (i *Issue) Key() IssueKey    { return i.key }
func (i *Issue) Title() string    { return i.title }
func (i *Issue) Type() IssueType  { return i.typ }
func (i *Issue) Status() StatusID { return i.status }

// Transition moves the issue to another status if the workflow allows it.
func (i *Issue) Transition(to StatusID, wf *Workflow) error {
	if !wf.CanTransition(i.status, to) {
		return ErrTransitionNotAllowed
	}
	from := i.status
	i.status = to
	i.record(IssueTransitioned{ID: i.id, From: from, To: to})
	return nil
}

// PullEvents returns and clears the recorded domain events.
func (i *Issue) PullEvents() []Event {
	ev := i.events
	i.events = nil
	return ev
}

func (i *Issue) record(e Event) { i.events = append(i.events, e) }
