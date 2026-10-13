package application

import (
	"context"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Import creates an issue from another tracker at the bottom of the backlog, keeping
// its status and history. It does not authorize: the Importer context checks admin
// rights before calling. Never expose it over HTTP directly.
func (s *Service) Import(ctx context.Context, project string, in domain.Imported) (*domain.Issue, error) {
	wf, err := s.workflowFor(ctx, project)
	if err != nil {
		return nil, err
	}
	// Validate with a throwaway key first so rejected rows do not burn issue numbers.
	probe, _ := domain.NewIssueKey(project, 1)
	if _, err := domain.NewImportedIssue("", probe, in, wf); err != nil {
		return nil, err
	}
	key, err := s.keys.Next(ctx, project)
	if err != nil {
		return nil, err
	}
	is, err := domain.NewImportedIssue(domain.IssueID(s.newID()), key, in, wf)
	if err != nil {
		return nil, err
	}
	last, err := s.issues.LastRank(ctx, project)
	if err != nil {
		return nil, err
	}
	is.Rerank(domain.RankBetween(last, ""))
	return is, s.save(ctx, "", is) // imported history arrives as issue.imported
}
