// Package domain is the Agile context: sprints (time boxes) over a project's backlog.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidSprint     = errors.New("sprint name must be 1-100 characters and goal at most 2000")
	ErrSprintState       = errors.New("sprint is not in a state that allows this")
	ErrSprintNotFound    = errors.New("sprint not found")
	ErrAnotherSprintLive = errors.New("project already has an active sprint")
	ErrSprintConflict    = errors.New("sprint was changed concurrently")
)

type (
	SprintID string
	State    string
)

const (
	StatePlanned State = "planned"
	StateActive  State = "active"
	StateClosed  State = "closed"
)

// Sprint is the aggregate root of the Agile context: planned → active → closed.
// It references its project by key and never holds issues; issues point at it.
type Sprint struct {
	id          SprintID
	project     string
	name, goal  string
	state       State
	startedAt   time.Time
	completedAt time.Time
	version     int // optimistic lock; 0 = never persisted
	events      []Event
}

func NewSprint(id SprintID, project, name, goal string) (*Sprint, error) {
	name, goal = strings.TrimSpace(name), strings.TrimSpace(goal)
	if name == "" || len([]rune(name)) > 100 || len([]rune(goal)) > 2000 {
		return nil, ErrInvalidSprint
	}
	s := &Sprint{id: id, project: project, name: name, goal: goal, state: StatePlanned}
	s.record(SprintCreated{ID: id, Project: project, Name: name})
	return s, nil
}

type Snapshot struct {
	ID          SprintID
	Project     string
	Name, Goal  string
	State       State
	StartedAt   time.Time
	CompletedAt time.Time
	Version     int
}

// Rehydrate rebuilds a persisted sprint without emitting events.
func Rehydrate(s Snapshot) *Sprint {
	return &Sprint{id: s.ID, project: s.Project, name: s.Name, goal: s.Goal, state: s.State,
		startedAt: s.StartedAt, completedAt: s.CompletedAt, version: s.Version}
}

// Version is the optimistic-locking version; MarkPersisted is called by repositories.
func (s *Sprint) Version() int   { return s.version }
func (s *Sprint) MarkPersisted() { s.version++ }

func (s *Sprint) ID() SprintID           { return s.id }
func (s *Sprint) Project() string        { return s.project }
func (s *Sprint) Name() string           { return s.name }
func (s *Sprint) Goal() string           { return s.goal }
func (s *Sprint) State() State           { return s.state }
func (s *Sprint) StartedAt() time.Time   { return s.startedAt }
func (s *Sprint) CompletedAt() time.Time { return s.completedAt }

// CanHoldIssues: issues may be planned into planned or active sprints only.
func (s *Sprint) CanHoldIssues() bool { return s.state != StateClosed }

// Start activates a planned sprint. "Only one active sprint per project" spans
// aggregates, so the application layer checks it (and the database enforces it).
func (s *Sprint) Start(at time.Time) error {
	if s.state != StatePlanned {
		return ErrSprintState
	}
	s.state, s.startedAt = StateActive, at
	s.record(SprintStarted{ID: s.id, Project: s.project})
	return nil
}

// Complete closes the active sprint; returning unfinished issues is a separate step.
func (s *Sprint) Complete(at time.Time) error {
	if s.state != StateActive {
		return ErrSprintState
	}
	s.state, s.completedAt = StateClosed, at
	s.record(SprintCompleted{ID: s.id, Project: s.project})
	return nil
}

func (s *Sprint) PullEvents() []Event {
	ev := s.events
	s.events = nil
	return ev
}

func (s *Sprint) record(e Event) { s.events = append(s.events, e) }

// Event is a domain event written to the outbox.
type Event interface{ EventName() string }

type SprintCreated struct {
	ID      SprintID `json:"id"`
	Project string   `json:"project"`
	Name    string   `json:"name"`
}

type SprintStarted struct {
	ID      SprintID `json:"id"`
	Project string   `json:"project"`
}

type SprintCompleted struct {
	ID      SprintID `json:"id"`
	Project string   `json:"project"`
}

func (SprintCreated) EventName() string   { return "sprint.created" }
func (SprintStarted) EventName() string   { return "sprint.started" }
func (SprintCompleted) EventName() string { return "sprint.completed" }

// Repository is the persistence port for sprints. Save inserts (version 0) or
// updates (version must match, else ErrSprintConflict) and writes events to the
// outbox atomically; a second active sprint in a project fails with ErrAnotherSprintLive.
type Repository interface {
	Save(ctx context.Context, s *Sprint, events []Event) error
	ByID(ctx context.Context, id SprintID) (*Sprint, error) // ErrSprintNotFound
	// ListByProject orders active, then planned (oldest first), then closed (newest first).
	ListByProject(ctx context.Context, project string) ([]*Sprint, error)
}
