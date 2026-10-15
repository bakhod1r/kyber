package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// RankMove places an issue directly after or before another issue of the same project.
type RankMove struct {
	After  string
	Before string
}

// Rank moves an issue in the backlog order; only the moved issue is rewritten.
func (s *Service) Rank(ctx context.Context, actor, rawKey string, m RankMove) (*domain.Issue, error) {
	if (m.After == "") == (m.Before == "") {
		return nil, ErrInvalidAnchor
	}
	is, err := s.load(ctx, actor, rawKey, PermEdit)
	if err != nil {
		return nil, err
	}
	anchorKey := m.After + m.Before
	key, err := domain.ParseIssueKey(anchorKey)
	if err != nil || key.Project() != is.Key().Project() {
		return nil, ErrInvalidAnchor
	}
	if key == is.Key() {
		return is, nil
	}
	anchor, err := s.issues.ByKey(ctx, key)
	if errors.Is(err, domain.ErrIssueNotFound) {
		return nil, ErrInvalidAnchor
	}
	if err != nil {
		return nil, err
	}
	project := is.Key().Project()
	var lo, hi domain.Rank
	if m.After != "" {
		lo = anchor.Rank()
		if hi, err = s.issues.NextRank(ctx, project, lo); err != nil {
			return nil, err
		}
		if hi == is.Rank() { // the next slot is the issue itself: look past it
			if hi, err = s.issues.NextRank(ctx, project, hi); err != nil {
				return nil, err
			}
		}
	} else {
		hi = anchor.Rank()
		if lo, err = s.issues.PrevRank(ctx, project, hi); err != nil {
			return nil, err
		}
		if lo == is.Rank() {
			if lo, err = s.issues.PrevRank(ctx, project, lo); err != nil {
				return nil, err
			}
		}
	}
	if lo != "" && hi != "" && lo >= hi {
		// Two issues share a rank after a concurrent move; the client retries.
		return nil, domain.ErrConcurrentModification
	}
	is.Rerank(domain.RankBetween(lo, hi))
	return is, s.save(ctx, actor, is)
}

// SprintTally counts the sprint's issues that are done and the rest, for the Agile context closing it.
func (s *Service) SprintTally(ctx context.Context, project, sprint string) (done, unfinished int, err error) {
	list, wf, err := s.sprintIssues(ctx, project, sprint)
	if err != nil {
		return 0, 0, err
	}
	for _, is := range list {
		if wf.IsDone(is.Status()) {
			done++
		} else {
			unfinished++
		}
	}
	return done, unfinished, nil
}

// OnSprintCompleted reacts to the Agile context's sprint.completed event by carrying every unfinished issue
// of the closed sprint over to the next sprint or the backlog (Jira semantics: done issues stay with the
// closed sprint). Re-delivery is harmless: carried-over issues are no longer in the sprint. When the target
// sprint has closed meanwhile, issues go to the backlog.
func (s *Service) OnSprintCompleted(ctx context.Context, m outbox.Message) error {
	var e struct {
		ID          string `json:"id"`
		Project     string `json:"project"`
		CarryOverTo string `json:"carry_over_to"`
		By          string `json:"by"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil || e.ID == "" || e.Project == "" {
		return nil // a malformed event must not block the relay
	}
	to := e.CarryOverTo
	if to != "" && s.sprints != nil {
		if err := s.sprints.CanHold(ctx, e.Project, to); errors.Is(err, ErrInvalidSprint) {
			to = ""
		} else if err != nil {
			return err
		}
	}
	list, wf, err := s.sprintIssues(ctx, e.Project, e.ID)
	if err != nil {
		return err
	}
	for _, is := range list {
		if wf.IsDone(is.Status()) {
			continue
		}
		is.CarryOver(domain.SprintID(to))
		if err := s.save(ctx, e.By, is); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) sprintIssues(ctx context.Context, project, sprint string) ([]*domain.Issue, *domain.Workflow, error) {
	sp := domain.SprintID(sprint)
	list, err := s.issues.ListByProject(ctx, project, domain.ListFilter{Sprint: &sp})
	if err != nil {
		return nil, nil, err
	}
	wf, err := s.workflowFor(ctx, project)
	return list, wf, err
}

// Peek loads an issue without authorization, for other bounded contexts reacting
// to events (they decide visibility themselves). Never expose it over HTTP.
func (s *Service) Peek(ctx context.Context, rawKey string) (*domain.Issue, error) {
	key, err := domain.ParseIssueKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	return s.issues.ByKey(ctx, key)
}

// Snapshot lists every issue of a project without authorization, for read models
// in other contexts (they authorize the caller themselves).
// ProjectWorkflow returns the project's workflow without authorization, for other bounded
// contexts (reports, boards) that have already authorized the actor.
func (s *Service) ProjectWorkflow(ctx context.Context, project string) (*domain.Workflow, error) {
	return s.workflowFor(ctx, project)
}

func (s *Service) Snapshot(ctx context.Context, project string) ([]*domain.Issue, error) {
	return s.issues.ListByProject(ctx, project, domain.ListFilter{})
}

// VisibleSnapshot is Snapshot narrowed to the issues the actor may see (security levels),
// for contexts that show issues to a person: reports, exports.
func (s *Service) VisibleSnapshot(ctx context.Context, actor, project string) ([]*domain.Issue, error) {
	list, err := s.Snapshot(ctx, project)
	if err != nil {
		return nil, err
	}
	return s.visible(ctx, actor, project, list)
}

// InVersion lists the project's issues planned into the version that the actor may see, for the
// Agile context's release views (it has authorized the actor for the project).
func (s *Service) InVersion(ctx context.Context, actor, project, version string) ([]*domain.Issue, error) {
	v := domain.VersionID(version)
	list, err := s.issues.ListByProject(ctx, project, domain.ListFilter{FixVersion: &v})
	if err != nil {
		return nil, err
	}
	return s.visible(ctx, actor, project, list)
}

// MoveFixVersion moves the project's issues from one version to another ("" = remove the
// version); with unresolvedOnly, resolved issues stay. It returns how many issues moved.
// Idempotent: issues already moved are no longer in `from`.
func (s *Service) MoveFixVersion(ctx context.Context, actor, project, from, to string, unresolvedOnly bool) (int, error) {
	v := domain.VersionID(from)
	list, err := s.issues.ListByProject(ctx, project, domain.ListFilter{FixVersion: &v})
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, is := range list {
		if unresolvedOnly && is.Resolution() != "" {
			continue
		}
		is.MoveToVersion(v, domain.VersionID(to))
		if err := s.save(ctx, actor, is); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// OnVersionEvent reacts to the Agile context's version.released and version.deleted events by
// moving the version's issues (eventual consistency between the two aggregates).
func (s *Service) OnVersionEvent(ctx context.Context, m outbox.Message) error {
	var e struct {
		ID               string `json:"id"`
		Project          string `json:"project"`
		MoveUnresolvedTo string `json:"move_unresolved_to"`
		MoveTo           string `json:"move_to"`
		By               string `json:"by"`
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil || e.ID == "" || e.Project == "" {
		return nil // a malformed event must not block the relay
	}
	switch {
	case m.Name == "version.released" && e.MoveUnresolvedTo != "":
		_, err := s.MoveFixVersion(ctx, e.By, e.Project, e.ID, e.MoveUnresolvedTo, true)
		return err
	case m.Name == "version.deleted":
		_, err := s.MoveFixVersion(ctx, e.By, e.Project, e.ID, e.MoveTo, false)
		return err
	}
	return nil
}
