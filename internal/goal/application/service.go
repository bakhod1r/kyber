// Package application holds the Goals context's use cases: a project's goals with their progress.
package application

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/goal/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("insufficient project role")
	// ErrInvalidQuery wraps why a goal's JQL cannot run.
	ErrInvalidQuery = errors.New("invalid goal query")
)

// Ports to the other contexts (anti-corruption layers live in infrastructure).
type (
	// Projects checks the actor's role on a project: viewing needs browse, changing needs write.
	Projects interface {
		Authorize(ctx context.Context, actor, project string, change bool) error
	}
	// Issues measures a query over the project's issues the actor may see (ACL to Search).
	Issues interface {
		Measure(ctx context.Context, actor, project, jql string) (domain.Progress, error)
	}
)

type Deps struct {
	Projects Projects
	Issues   Issues
	Goals    domain.Repository
	NewID    func() string
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// View is a goal with its progress as the actor sees it.
type View struct {
	Goal     *domain.Goal
	Progress domain.Progress
}

func (s *Service) view(ctx context.Context, actor string, g *domain.Goal) (View, error) {
	if g.JQL == "" {
		return View{Goal: g}, nil
	}
	p, err := s.d.Issues.Measure(ctx, actor, g.Project, g.JQL)
	if errors.Is(err, ErrInvalidQuery) {
		err = nil // the query broke later (e.g. a custom field was removed): no progress
	}
	return View{Goal: g, Progress: p}, err
}

// Goals lists the project's goals with their progress.
func (s *Service) Goals(ctx context.Context, actor, project string) ([]View, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	list, err := s.d.Goals.ForProject(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(list))
	for _, g := range list {
		v, err := s.view(ctx, actor, g)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// checkQuery rejects a query that cannot run.
func (s *Service) checkQuery(ctx context.Context, actor string, g *domain.Goal) (View, error) {
	if g.JQL == "" {
		return View{Goal: g}, nil
	}
	p, err := s.d.Issues.Measure(ctx, actor, g.Project, g.JQL)
	return View{Goal: g, Progress: p}, err
}

// Create adds a goal owned by the actor.
func (s *Service) Create(ctx context.Context, actor, project, name, target, jql string) (View, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, true); err != nil {
		return View{}, err
	}
	g, err := domain.NewGoal(domain.GoalID(s.d.NewID()), project, actor, name, target, jql)
	if err != nil {
		return View{}, err
	}
	v, err := s.checkQuery(ctx, actor, g)
	if err != nil {
		return View{}, err
	}
	return v, s.d.Goals.Save(ctx, g)
}

// Change edits a goal loaded at version and reports its status.
type Change struct {
	Version           int
	Name, Target, JQL string
	Status            domain.Status
}

// load finds a goal and checks the actor's role on its project.
func (s *Service) load(ctx context.Context, actor, id string) (*domain.Goal, error) {
	g, err := s.d.Goals.ByID(ctx, domain.GoalID(id))
	if err != nil {
		return nil, err
	}
	if err := s.d.Projects.Authorize(ctx, actor, g.Project, true); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrGoalNotFound // goals of hidden projects stay hidden
		}
		return nil, err
	}
	return g, nil
}

func (s *Service) Update(ctx context.Context, actor, id string, c Change) (View, error) {
	g, err := s.load(ctx, actor, id)
	if err != nil {
		return View{}, err
	}
	if g.Version != c.Version {
		return View{}, domain.ErrGoalConflict
	}
	if err := g.Redefine(c.Name, c.Target, c.JQL); err != nil {
		return View{}, err
	}
	if err := g.Report(c.Status); err != nil {
		return View{}, err
	}
	v, err := s.checkQuery(ctx, actor, g)
	if err != nil {
		return View{}, err
	}
	return v, s.d.Goals.Save(ctx, g)
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	g, err := s.load(ctx, actor, id)
	if err != nil {
		return err
	}
	return s.d.Goals.Delete(ctx, g.ID)
}
