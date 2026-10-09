package app

import (
	"context"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Import creates an issue from another tracker at the bottom of the backlog, keeping
// its status and history. It does not authorize: the Importer context checks admin
// rights before calling. Never expose it over HTTP directly.
func (s *Service) Import(ctx context.Context, project string, in domain.Imported) (*domain.Issue, error) {
	// Validate with a throwaway key first so rejected rows do not burn issue numbers.
	probe, _ := domain.NewIssueKey(project, 1)
	if _, err := domain.NewImportedIssue("", probe, in); err != nil {
		return nil, err
	}
	key, err := s.keys.Next(ctx, project)
	if err != nil {
		return nil, err
	}
	is, err := domain.NewImportedIssue(domain.IssueID(s.newID()), key, in)
	if err != nil {
		return nil, err
	}
	last, err := s.issues.LastRank(ctx, project)
	if err != nil {
		return nil, err
	}
	is.Rerank(domain.RankBetween(last, ""))
	return is, s.save(ctx, is)
}
