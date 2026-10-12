package repotest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// OtherWorkspace is a second workspace adapters must accept (the Postgres factory inserts it
// with its own KYB project) for the tenant isolation check.
const OtherWorkspace = "00000000-0000-4000-8000-0000000000aa"

// WorkflowFactory returns an empty workflow repository; project KYB exists in both workspaces.
type WorkflowFactory func(t *testing.T) domain.WorkflowRepository

func review() domain.WorkflowDef {
	return domain.WorkflowDef{Initial: "open", Statuses: []domain.Status{
		{ID: "open", Name: "Open", Category: domain.CategoryToDo},
		{ID: "review", Name: "Review", Category: domain.CategoryInProgress},
		{ID: "closed", Name: "Closed", Category: domain.CategoryDone},
	}, Transitions: []domain.Transition{{From: "open", To: "review"}, {From: "review", To: "closed"}}}
}

func RunWorkflows(t *testing.T, newRepo WorkflowFactory) {
	ctx := context.Background()
	t.Run("missing means default", func(t *testing.T) {
		repo := newRepo(t)
		if _, err := repo.ByProject(ctx, "KYB"); !errors.Is(err, domain.ErrWorkflowNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("save, load and replace", func(t *testing.T) {
		repo := newRepo(t)
		if err := repo.Save(ctx, "KYB", review()); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ByProject(ctx, "KYB")
		if err != nil || !reflect.DeepEqual(got, review()) {
			t.Fatalf("got %+v, %v", got, err)
		}
		def := domain.DefaultWorkflowDef()
		if err := repo.Save(ctx, "KYB", def); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.ByProject(ctx, "KYB"); !reflect.DeepEqual(got, def) {
			t.Fatalf("replaced = %+v", got)
		}
	})
	t.Run("workspaces are isolated", func(t *testing.T) {
		repo := newRepo(t)
		if err := repo.Save(ctx, "KYB", review()); err != nil {
			t.Fatal(err)
		}
		other := tenant.With(ctx, OtherWorkspace)
		if _, err := repo.ByProject(other, "KYB"); !errors.Is(err, domain.ErrWorkflowNotFound) {
			t.Fatalf("leaked across workspaces: %v", err)
		}
	})
}
