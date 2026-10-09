package domain

import (
	"errors"
	"strings"
)

var (
	ErrEmptyTitle       = errors.New("issue title must not be empty")
	ErrInvalidIssueType = errors.New("issue type must be one of epic, story, task, bug, subtask")
	ErrIssueNotFound    = errors.New("issue not found")
	// ErrConcurrentModification means the issue was changed by someone else since it was loaded.
	ErrConcurrentModification = errors.New("issue was modified concurrently")
)

type IssueID string

type IssueType string

const (
	TypeEpic    IssueType = "epic"
	TypeStory   IssueType = "story"
	TypeTask    IssueType = "task"
	TypeBug     IssueType = "bug"
	TypeSubtask IssueType = "subtask"
)

// NormalizeTitle trims a title and rejects blank ones.
func NormalizeTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrEmptyTitle
	}
	return s, nil
}

// ParseIssueType validates a raw issue type.
func ParseIssueType(s string) (IssueType, error) {
	switch t := IssueType(s); t {
	case TypeEpic, TypeStory, TypeTask, TypeBug, TypeSubtask:
		return t, nil
	}
	return "", ErrInvalidIssueType
}

// Issue is the aggregate root of the Issue Tracking context.
type Issue struct {
	id      IssueID
	key     IssueKey
	title   string
	typ     IssueType
	status  StatusID
	version int // 0 = never persisted
	events  []Event
}

func NewIssue(id IssueID, key IssueKey, title string, typ IssueType, wf *Workflow) (*Issue, error) {
	title, err := NormalizeTitle(title)
	if err != nil {
		return nil, err
	}
	is := &Issue{id: id, key: key, title: title, typ: typ, status: wf.Initial()}
	is.record(IssueCreated{ID: id, Key: key, Type: typ, Title: title})
	return is, nil
}

// Rehydrate rebuilds a persisted issue without emitting events (repository use only).
func Rehydrate(id IssueID, key IssueKey, title string, typ IssueType, status StatusID, version int) *Issue {
	return &Issue{id: id, key: key, title: title, typ: typ, status: status, version: version}
}

// Version is the optimistic-locking version; 0 means not yet persisted.
func (i *Issue) Version() int { return i.version }

// MarkPersisted is called by repositories after a successful save.
func (i *Issue) MarkPersisted() { i.version++ }

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
	i.record(IssueTransitioned{ID: i.id, Key: i.key, From: from, To: to})
	return nil
}

// PullEvents returns and clears the recorded domain events.
func (i *Issue) PullEvents() []Event {
	ev := i.events
	i.events = nil
	return ev
}

func (i *Issue) record(e Event) { i.events = append(i.events, e) }
