// Package application holds the Automation context's use cases: managing a project's rules and running them
// when issue events arrive through the outbox.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/automation/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("insufficient project role")
	// ErrInvalidCondition wraps why a rule's JQL cannot run.
	ErrInvalidCondition = errors.New("invalid rule condition")
)

// Ports to the other contexts (anti-corruption layers live in infrastructure).
type (
	// Projects checks the actor's role: reading rules needs browse, changing them needs admin.
	Projects interface {
		Authorize(ctx context.Context, actor, project string, admin bool) error
	}
	// Issues performs actions as actor (ACL to Issue Tracking); its permission checks apply.
	Issues interface {
		Transition(ctx context.Context, actor, key, status string) error
		// Assign takes a user id, "reporter" or "unassigned".
		Assign(ctx context.Context, actor, key, who string) error
		AddLabel(ctx context.Context, actor, key, label string) error
		Comment(ctx context.Context, actor, key, body string) error
	}
	// Conditions checks JQL (ACL to Search).
	Conditions interface {
		Validate(ctx context.Context, actor, project, jql string) error // ErrInvalidCondition
		Matches(ctx context.Context, actor, project, key, jql string) (bool, error)
	}
)

type Deps struct {
	Projects   Projects
	Issues     Issues
	Conditions Conditions
	Rules      domain.Repository
	NewID      func() string
	Now        func() time.Time
	Log        *slog.Logger
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

// recentRuns is how many runs the audit trail shows.
const recentRuns = 50

func (s *Service) Rules(ctx context.Context, actor, project string) ([]*domain.Rule, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	return s.d.Rules.ForProject(ctx, project)
}

func (s *Service) Runs(ctx context.Context, actor, project string) ([]domain.Run, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	return s.d.Rules.Runs(ctx, project, recentRuns)
}

func (s *Service) checkCondition(ctx context.Context, actor, project string, spec domain.RuleSpec) error {
	if strings.TrimSpace(spec.Condition) == "" {
		return nil
	}
	return s.d.Conditions.Validate(ctx, actor, project, spec.Condition)
}

// Create adds a rule that runs as its creator; project admins only.
func (s *Service) Create(ctx context.Context, actor, project string, spec domain.RuleSpec) (*domain.Rule, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, true); err != nil {
		return nil, err
	}
	r, err := domain.NewRule(domain.RuleID(s.d.NewID()), project, actor, spec)
	if err != nil {
		return nil, err
	}
	if err := s.checkCondition(ctx, actor, project, r.RuleSpec); err != nil {
		return nil, err
	}
	return r, s.d.Rules.Save(ctx, r)
}

// Update redefines a rule and turns it on or off (enabled nil = unchanged); project admins only.
func (s *Service) Update(ctx context.Context, actor, id string, version int, spec domain.RuleSpec, enabled *bool) (*domain.Rule, error) {
	r, err := s.adminRule(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if r.Version != version {
		return nil, domain.ErrRuleConflict
	}
	if err := r.Redefine(spec); err != nil {
		return nil, err
	}
	if err := s.checkCondition(ctx, actor, r.Project, r.RuleSpec); err != nil {
		return nil, err
	}
	if enabled != nil {
		if *enabled {
			r.Enable()
		} else {
			r.Disable()
		}
	}
	return r, s.d.Rules.Save(ctx, r)
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	r, err := s.adminRule(ctx, actor, id)
	if err != nil {
		return err
	}
	return s.d.Rules.Delete(ctx, r.ID)
}

func (s *Service) adminRule(ctx context.Context, actor, id string) (*domain.Rule, error) {
	r, err := s.d.Rules.ByID(ctx, domain.RuleID(id))
	if err != nil {
		return nil, err
	}
	if err := s.d.Projects.Authorize(ctx, actor, r.Project, true); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrRuleNotFound
		}
		return nil, err
	}
	return r, nil
}

// OnEvent runs the project's matching rules for an issue event (an outbox handler). A rule's failure is
// recorded in its runs, not retried: the event is not the rule's to block.
func (s *Service) OnEvent(ctx context.Context, m outbox.Message) error {
	ev, ok := domain.EventFrom(m.Name, m.Actor, m.Payload)
	if !ok {
		return nil
	}
	rules, err := s.d.Rules.ForProject(ctx, ev.Project)
	if err != nil {
		return err // storage trouble: let the relay retry
	}
	for _, r := range rules {
		if !r.Matches(ev) {
			continue
		}
		run := domain.Run{Rule: r.ID, Project: r.Project, IssueKey: ev.IssueKey, At: s.d.Now()}
		done, err := s.run(ctx, r, ev)
		run.OK, run.Detail = err == nil, done
		if err != nil {
			run.Detail = err.Error()
			s.d.Log.WarnContext(ctx, "automation rule failed", "rule", r.ID, "issue", ev.IssueKey, "err", err)
		}
		if done == "" && err == nil {
			continue // the condition did not match: nothing to record
		}
		if err := s.d.Rules.Record(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

// run checks the condition and performs the actions in order, stopping at the first failure. done describes
// what was done ("" when the condition did not match).
func (s *Service) run(ctx context.Context, r *domain.Rule, ev domain.Event) (string, error) {
	if r.Condition != "" {
		ok, err := s.d.Conditions.Matches(ctx, r.Owner, r.Project, ev.IssueKey, r.Condition)
		if err != nil || !ok {
			return "", err
		}
	}
	var done []string
	for _, a := range r.Actions {
		var err error
		switch a.Kind {
		case domain.ActionTransition:
			err = s.d.Issues.Transition(ctx, r.Owner, ev.IssueKey, a.Value)
		case domain.ActionAssign:
			err = s.d.Issues.Assign(ctx, r.Owner, ev.IssueKey, a.Value)
		case domain.ActionAddLabel:
			err = s.d.Issues.AddLabel(ctx, r.Owner, ev.IssueKey, a.Value)
		case domain.ActionComment:
			err = s.d.Issues.Comment(ctx, r.Owner, ev.IssueKey, a.Value)
		}
		if err != nil {
			return strings.Join(done, ", "), fmt.Errorf("%s %q: %w", a.Kind, a.Value, err)
		}
		done = append(done, fmt.Sprintf("%s %s", a.Kind, a.Value))
	}
	return strings.Join(done, ", "), nil
}
