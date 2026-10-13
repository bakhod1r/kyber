package memory_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/memory"
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

func TestWorkflowContract(t *testing.T) {
	repotest.RunWorkflows(t, func(t *testing.T) domain.WorkflowRepository { return memory.NewWorkflowRepository() })
}

func TestHistoryContract(t *testing.T) {
	repotest.RunHistory(t, func(t *testing.T) domain.HistoryRepository { return memory.NewHistoryRepository() })
}

func TestLinkContract(t *testing.T) {
	repotest.RunLinks(t, func(t *testing.T) (domain.LinkRepository, [3]*domain.Issue, func() []string) {
		r := memory.NewRepository()
		var is [3]*domain.Issue
		for n := range 3 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
			_ = r.Save(context.Background(), is[n], nil)
		}
		return memory.NewLinkRepository(r), is, r.OutboxNames
	})
}

func TestWatcherContract(t *testing.T) {
	repotest.RunWatchers(t, func(t *testing.T) (domain.WatcherRepository, [2]*domain.Issue) {
		var is [2]*domain.Issue
		for n := range 2 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
		}
		return memory.NewWatcherRepository(), is
	})
}

func TestWorklogContract(t *testing.T) {
	repotest.RunWorklogs(t, func(t *testing.T) (domain.WorklogRepository, [2]*domain.Issue) {
		var is [2]*domain.Issue
		for n := range 2 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
		}
		return memory.NewWorklogRepository(), is
	})
}
