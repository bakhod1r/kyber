package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
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

// ReturnUnfinished carries every issue of the sprint that is not done over to `to` — the
// backlog ("") or the next sprint (Jira semantics: done issues stay with the closed sprint). It is called
// by the Agile context after it has authorized the actor.
func (s *Service) ReturnUnfinished(ctx context.Context, actor, project, sprint, to string) (completed, returned int, err error) {
	sp := domain.SprintID(sprint)
	list, err := s.issues.ListByProject(ctx, project, domain.ListFilter{Sprint: &sp})
	if err != nil {
		return 0, 0, err
	}
	wf, err := s.workflowFor(ctx, project)
	if err != nil {
		return 0, 0, err
	}
	for _, is := range list {
		if wf.IsDone(is.Status()) {
			completed++
			continue
		}
		is.CarryOver(domain.SprintID(to))
		if err := s.save(ctx, actor, is); err != nil {
			return completed, returned, err
		}
		returned++
	}
	return completed, returned, nil
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
// version); with unresolvedOnly, resolved issues stay. It returns how many issues moved. Called
// by the Agile context when releasing or deleting a version, after authorizing the actor.
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
		is.ReplaceFixVersion(v, domain.VersionID(to))
		if err := s.save(ctx, actor, is); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}
