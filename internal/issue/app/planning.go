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
	is, err := s.load(ctx, actor, rawKey, true)
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
	return is, s.save(ctx, is)
}

// ReturnUnfinished moves every issue of the sprint that is not done back to the
// backlog (Jira semantics: done issues stay with the closed sprint). It is called
// by the Agile context after it has authorized the actor.
func (s *Service) ReturnUnfinished(ctx context.Context, project, sprint string) (completed, returned int, err error) {
	sp := domain.SprintID(sprint)
	list, err := s.issues.ListByProject(ctx, project, domain.ListFilter{Sprint: &sp})
	if err != nil {
		return 0, 0, err
	}
	for _, is := range list {
		if is.Status() == domain.StatusDone {
			completed++
			continue
		}
		is.MoveToSprint("")
		if err := s.save(ctx, is); err != nil {
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
func (s *Service) Snapshot(ctx context.Context, project string) ([]*domain.Issue, error) {
	return s.issues.ListByProject(ctx, project, domain.ListFilter{})
}
