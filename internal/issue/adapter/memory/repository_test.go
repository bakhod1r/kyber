package memory_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		r := memory.NewRepository()
		return r, r.OutboxNames
	})
}

func TestCommentContract(t *testing.T) {
	repotest.RunComments(t, func(t *testing.T) (domain.CommentRepository, *domain.Issue, func() []string) {
		r := memory.NewRepository()
		key, _ := domain.NewIssueKey("KYB", 1)
		is, _ := domain.NewIssue("00000000-0000-4000-8000-000000000001", key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
		_ = r.Save(context.Background(), is, nil)
		return memory.NewCommentRepository(r), is, r.OutboxNames
	})
}
