package issueacl

import (
	"context"

	agileapp "github.com/bakhod1r/kyber/internal/agile/application"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
)

// IssueSource is the part of Issue Tracking the Agile context uses for sprints and versions.
type IssueSource interface {
	Source
	SprintTally(ctx context.Context, project, sprint string) (done, unfinished int, err error)
	InVersion(ctx context.Context, actor, project, version string) ([]*issuedomain.Issue, error)
}

// Issues adapts Issue Tracking to the Agile context's Issues port.
type Issues struct{ src IssueSource }

func NewIssues(src IssueSource) Issues { return Issues{src: src} }

func (a Issues) SprintTally(ctx context.Context, project, sprint string) (int, int, error) {
	return a.src.SprintTally(ctx, project, sprint)
}

func (a Issues) VersionIssues(ctx context.Context, actor, project, version string) ([]agileapp.VersionIssue, error) {
	list, err := a.src.InVersion(ctx, actor, project, version)
	if err != nil {
		return nil, err
	}
	wf, err := a.src.ProjectWorkflow(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]agileapp.VersionIssue, 0, len(list))
	for _, is := range list {
		out = append(out, agileapp.VersionIssue{Key: is.Key().String(), Title: is.Title(), Type: string(is.Type()),
			Done: wf.IsDone(is.Status()), Resolution: string(is.Resolution())})
	}
	return out, nil
}
