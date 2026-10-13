// Package repotest is the contract every form repository must pass.
package repotest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bakhod1r/kyber/internal/form/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// OtherWorkspace must exist besides the default one.
const OtherWorkspace = "00000000-0000-4000-8000-0000000000ad"

func id(n int) domain.FormID {
	return domain.FormID("92000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, project, name string) *domain.Form {
		f, err := domain.NewForm(id(n), project, name, "Intro", "bug", []domain.QuestionInput{{Label: "Steps", Required: true, Long: true}, {Label: "Browser"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, f); err != nil || f.Version != 1 {
			t.Fatalf("save %s: %v", name, err)
		}
		return f
	}
	bug := mk(1, "KYB", "Bug report")
	mk(2, "KYB", "Access request")
	mk(3, "OPS", "Incident")

	list, err := repo.ForProject(ctx, "KYB")
	if err != nil || len(list) != 2 || list[0].Name != "Access request" || list[1].Name != "Bug report" {
		t.Fatalf("for project = %v", err)
	}
	got, err := repo.ByID(ctx, bug.ID)
	if err != nil || !reflect.DeepEqual(got, bug) {
		t.Fatalf("by id = %+v %v", got, err)
	}
	if err := got.Redesign("Bug", "", "task", nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, got); err != nil || got.Version != 2 {
		t.Fatalf("update: %v", err)
	}
	if again, _ := repo.ByID(ctx, bug.ID); again.Name != "Bug" || again.IssueType != "task" || len(again.Questions) != 0 || again.Intro != "" {
		t.Fatalf("updated = %+v", again)
	}
	stale := *bug // version 1
	if err := repo.Save(ctx, &stale); !errors.Is(err, domain.ErrFormConflict) {
		t.Fatalf("stale save: %v", err)
	}
	dup, _ := domain.NewForm(bug.ID, "KYB", "Dup", "", "task", nil)
	if err := repo.Save(ctx, dup); !errors.Is(err, domain.ErrFormConflict) {
		t.Fatalf("duplicate insert: %v", err)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.ByID(other, bug.ID); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("other workspace reads: %v", err)
	}
	if l, _ := repo.ForProject(other, "KYB"); len(l) != 0 {
		t.Fatalf("other workspace lists: %v", l)
	}
	if err := repo.Delete(other, bug.ID); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("other workspace deletes: %v", err)
	}
	if _, err := repo.ByID(ctx, "not-an-id"); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if err := repo.Delete(ctx, "not-an-id"); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("bad id delete: %v", err)
	}
	if err := repo.Delete(ctx, bug.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByID(ctx, bug.ID); !errors.Is(err, domain.ErrFormNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}
