// Package app holds the Agile context use cases.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("insufficient project role")
	ErrInvalidSprint   = errors.New("sprint must be a planned or active sprint of this project")
)

// Access is the anti-corruption port to the Project context's membership rules.
type Access interface {
	Authorize(ctx context.Context, actor, project string, write bool) error
}

// Issues is the anti-corruption port to Issue Tracking.
type Issues interface {
	ReturnUnfinished(ctx context.Context, project, sprint string) (completed, returned int, err error)
}

type Deps struct {
	Sprints domain.Repository
	Issues  Issues
	Access  Access
	NewID   func() string
	Now     func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

func (s *Service) Create(ctx context.Context, actor, project, name, goal string) (*domain.Sprint, error) {
	if err := s.d.Access.Authorize(ctx, actor, project, true); err != nil {
		return nil, err
	}
	sp, err := domain.NewSprint(domain.SprintID(s.d.NewID()), project, name, goal)
	if err != nil {
		return nil, err
	}
	return sp, s.d.Sprints.Save(ctx, sp, sp.PullEvents())
}

func (s *Service) List(ctx context.Context, actor, project string) ([]*domain.Sprint, error) {
	if err := s.d.Access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	return s.d.Sprints.ListByProject(ctx, project)
}

// load finds a sprint the actor may see; non-members get ErrSprintNotFound.
func (s *Service) load(ctx context.Context, actor, id string, write bool) (*domain.Sprint, error) {
	sp, err := s.d.Sprints.ByID(ctx, domain.SprintID(id))
	if err != nil {
		return nil, err
	}
	if err := s.d.Access.Authorize(ctx, actor, sp.Project(), write); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrSprintNotFound
		}
		return nil, err
	}
	return sp, nil
}

// DefaultSprintLength is used when Start is given no end date (Jira's default).
const DefaultSprintLength = 14 * 24 * time.Hour

func (s *Service) Start(ctx context.Context, actor, id string, endsAt time.Time) (*domain.Sprint, error) {
	sp, err := s.load(ctx, actor, id, true)
	if err != nil {
		return nil, err
	}
	all, err := s.d.Sprints.ListByProject(ctx, sp.Project())
	if err != nil {
		return nil, err
	}
	for _, other := range all {
		if other.ID() != sp.ID() && other.State() == domain.StateActive {
			return nil, domain.ErrAnotherSprintLive
		}
	}
	now := s.d.Now()
	if endsAt.IsZero() {
		endsAt = now.Add(DefaultSprintLength)
	}
	if err := sp.Start(now, endsAt); err != nil {
		return nil, err
	}
	return sp, s.d.Sprints.Save(ctx, sp, sp.PullEvents()) // the DB index catches a concurrent start
}

// Completion reports what happened to the sprint's issues.
type Completion struct {
	Sprint    *domain.Sprint
	Completed int // done issues kept with the closed sprint
	Returned  int // unfinished issues moved back to the backlog
}

// Complete returns unfinished issues to the backlog first and only then closes
// the sprint, so a failure leaves it active and the action can simply be retried.
func (s *Service) Complete(ctx context.Context, actor, id string) (Completion, error) {
	sp, err := s.load(ctx, actor, id, true)
	if err != nil {
		return Completion{}, err
	}
	if sp.State() != domain.StateActive {
		return Completion{}, domain.ErrSprintState
	}
	done, returned, err := s.d.Issues.ReturnUnfinished(ctx, sp.Project(), string(sp.ID()))
	if err != nil {
		return Completion{}, err
	}
	if err := sp.Complete(s.d.Now()); err != nil {
		return Completion{}, err
	}
	if err := s.d.Sprints.Save(ctx, sp, sp.PullEvents()); err != nil {
		return Completion{}, err
	}
	return Completion{Sprint: sp, Completed: done, Returned: returned}, nil
}

// CanHold implements Issue Tracking's Sprints port.
func (s *Service) CanHold(ctx context.Context, project, id string) error {
	sp, err := s.d.Sprints.ByID(ctx, domain.SprintID(id))
	if errors.Is(err, domain.ErrSprintNotFound) {
		return ErrInvalidSprint
	}
	if err != nil {
		return err
	}
	if sp.Project() != project || !sp.CanHoldIssues() {
		return ErrInvalidSprint
	}
	return nil
}
