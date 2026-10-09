package domain

import (
	"errors"
	"math"
	"strings"
)

var (
	ErrEmptyTitle       = errors.New("issue title must not be empty")
	ErrInvalidIssueType = errors.New("issue type must be one of epic, story, task, bug, subtask")
	ErrIssueNotFound    = errors.New("issue not found")
	// ErrConcurrentModification means the issue was changed by someone else since it was loaded.
	ErrConcurrentModification = errors.New("issue was modified concurrently")
	ErrInvalidPriority        = errors.New("priority must be one of lowest, low, medium, high, highest")
	ErrDescriptionTooLong     = errors.New("description must be at most 20000 characters")
	ErrInvalidEstimate        = errors.New("estimate must be 0-999 story points in steps of 0.1")
)

// Points are story points in tenths (2.5 points = 25), so sums are exact.
type Points int

// ParsePoints converts client points (at most one decimal) to Points.
func ParsePoints(f float64) (Points, error) {
	t := math.Round(f * 10)
	if f < 0 || f > 999 || math.Abs(f*10-t) > 1e-9 {
		return 0, ErrInvalidEstimate
	}
	return Points(t), nil
}

// Float returns the points as a decimal number (for JSON).
func (p Points) Float() float64 { return float64(p) / 10 }

// SprintID identifies a sprint of the Agile context ("" = backlog).
type SprintID string

// UserID identifies a user of the Identity context (published language).
type UserID string

type Priority string

const (
	PriorityLowest  Priority = "lowest"
	PriorityLow     Priority = "low"
	PriorityMedium  Priority = "medium"
	PriorityHigh    Priority = "high"
	PriorityHighest Priority = "highest"
)

func ParsePriority(s string) (Priority, error) {
	switch p := Priority(s); p {
	case PriorityLowest, PriorityLow, PriorityMedium, PriorityHigh, PriorityHighest:
		return p, nil
	}
	return "", ErrInvalidPriority
}

const maxDescription = 20_000

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
	id          IssueID
	key         IssueKey
	title       string
	typ         IssueType
	status      StatusID
	description string
	priority    Priority
	assignee    UserID   // "" = unassigned
	reporter    UserID   // creator; "" for issues created before 0.6
	estimate    *Points  // nil = unestimated
	rank        Rank     // backlog order
	sprint      SprintID // "" = backlog
	version     int      // 0 = never persisted
	events      []Event
}

func NewIssue(id IssueID, key IssueKey, title string, typ IssueType, reporter UserID, wf *Workflow) (*Issue, error) {
	title, err := NormalizeTitle(title)
	if err != nil {
		return nil, err
	}
	is := &Issue{id: id, key: key, title: title, typ: typ, status: wf.Initial(), priority: PriorityMedium, reporter: reporter}
	is.record(IssueCreated{ID: id, Key: key, Type: typ, Title: title, Reporter: reporter})
	return is, nil
}

// Snapshot is the persisted state of an issue.
type Snapshot struct {
	ID          IssueID
	Key         IssueKey
	Title       string
	Type        IssueType
	Status      StatusID
	Description string
	Priority    Priority
	Assignee    UserID
	Reporter    UserID
	Estimate    *Points
	Rank        Rank
	Sprint      SprintID
	Version     int
}

// Rehydrate rebuilds a persisted issue without emitting events (repository use only).
func Rehydrate(s Snapshot) *Issue {
	if s.Priority == "" {
		s.Priority = PriorityMedium
	}
	return &Issue{id: s.ID, key: s.Key, title: s.Title, typ: s.Type, status: s.Status,
		description: s.Description, priority: s.Priority, assignee: s.Assignee, reporter: s.Reporter, estimate: s.Estimate, rank: s.Rank, sprint: s.Sprint,
		version: s.Version}
}

// Version is the optimistic-locking version; 0 means not yet persisted.
func (i *Issue) Version() int { return i.version }

// MarkPersisted is called by repositories after a successful save.
func (i *Issue) MarkPersisted() { i.version++ }

func (i *Issue) ID() IssueID         { return i.id }
func (i *Issue) Key() IssueKey       { return i.key }
func (i *Issue) Title() string       { return i.title }
func (i *Issue) Type() IssueType     { return i.typ }
func (i *Issue) Status() StatusID    { return i.status }
func (i *Issue) Description() string { return i.description }
func (i *Issue) Priority() Priority  { return i.priority }
func (i *Issue) Assignee() UserID    { return i.assignee }
func (i *Issue) Rank() Rank          { return i.rank }
func (i *Issue) Reporter() UserID    { return i.reporter }

// Estimate returns the story points and whether the issue is estimated.
func (i *Issue) Estimate() (Points, bool) {
	if i.estimate == nil {
		return 0, false
	}
	return *i.estimate, true
}

// SetEstimate sets (or with nil clears) the story points.
func (i *Issue) SetEstimate(p *Points) {
	old, had := i.Estimate()
	if (p == nil && !had) || (p != nil && had && *p == old) {
		return
	}
	var from, to *float64
	if had {
		f := old.Float()
		from = &f
	}
	if p != nil {
		v := *p
		i.estimate = &v
		f := v.Float()
		to = &f
	} else {
		i.estimate = nil
	}
	i.record(IssueEstimated{ID: i.id, Key: i.key, From: from, To: to})
}
func (i *Issue) Sprint() SprintID { return i.sprint }

// Rerank places the issue at r in the project's backlog order.
func (i *Issue) Rerank(r Rank) { i.rank = r }

// MoveToSprint puts the issue into a sprint, or with "" back into the backlog.
// Whether the sprint may hold the issue is checked by the application layer.
func (i *Issue) MoveToSprint(s SprintID) {
	if s == i.sprint {
		return
	}
	from := i.sprint
	i.sprint = s
	i.record(IssueSprintChanged{ID: i.id, Key: i.key, From: from, To: s})
}

// Changes lists the fields to edit; nil means "leave unchanged".
type Changes struct {
	Title       *string
	Description *string
	Priority    *Priority
}

// Edit validates every change first and applies all or none.
func (i *Issue) Edit(c Changes) error {
	title, desc, prio := i.title, i.description, i.priority
	if c.Title != nil {
		t, err := NormalizeTitle(*c.Title)
		if err != nil {
			return err
		}
		title = t
	}
	if c.Description != nil {
		if len([]rune(*c.Description)) > maxDescription {
			return ErrDescriptionTooLong
		}
		desc = *c.Description
	}
	if c.Priority != nil {
		p, err := ParsePriority(string(*c.Priority))
		if err != nil {
			return err
		}
		prio = p
	}
	var fields []string
	if title != i.title {
		fields = append(fields, "title")
	}
	if desc != i.description {
		fields = append(fields, "description")
	}
	if prio != i.priority {
		fields = append(fields, "priority")
	}
	if len(fields) == 0 {
		return nil
	}
	i.title, i.description, i.priority = title, desc, prio
	i.record(IssueEdited{ID: i.id, Key: i.key, Fields: fields})
	return nil
}

// Assign sets (or with "" clears) the assignee; by is the acting user, recorded in the
// event so that notifications can name them and skip them. Membership is checked by the application layer.
func (i *Issue) Assign(u, by UserID) {
	if u == i.assignee {
		return
	}
	from := i.assignee
	i.assignee = u
	i.record(IssueAssigned{ID: i.id, Key: i.key, From: from, To: u, By: by})
}

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
