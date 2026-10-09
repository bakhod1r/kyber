// Package projectacl is the anti-corruption layer from Issue Tracking to the Project context.
package projectacl

import (
	"context"
	"errors"

	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type Sequencer interface {
	NextIssueNumber(ctx context.Context, projectKey string) (int, error)
}

type Allocator struct{ seq Sequencer }

func New(seq Sequencer) *Allocator { return &Allocator{seq: seq} }

func (a *Allocator) Next(ctx context.Context, project string) (domain.IssueKey, error) {
	n, err := a.seq.NextIssueNumber(ctx, project)
	if errors.Is(err, projectdomain.ErrProjectNotFound) {
		return domain.IssueKey{}, issueapp.ErrProjectNotFound
	}
	if err != nil {
		return domain.IssueKey{}, err
	}
	return domain.NewIssueKey(project, n)
}
