// Package repotest is the contract every saved-filter repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/search/domain"
)

// Users and workspace the factory must create.
const (
	Ann            = "61000000-0000-4000-8000-000000000001"
	Ben            = "61000000-0000-4000-8000-000000000002"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000aa"
)

func id(n int) domain.FilterID {
	return domain.FilterID("62000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

func RunFilters(t *testing.T, repo domain.FilterRepository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, owner, name string, shared bool) *domain.Filter {
		f, err := domain.NewFilter(id(n), owner, name, "type = bug", shared)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveFilter(ctx, f); err != nil || f.Version != 1 {
			t.Fatalf("save %s: %v", name, err)
		}
		return f
	}
	mine := mk(1, Ann, "Zeta mine", false)
	mk(2, Ann, "Alpha mine", false)
	mk(3, Ben, "Ben private", false)
	mk(4, Ben, "Ben shared", true)

	list, err := repo.FiltersFor(ctx, Ann)
	names := ""
	for _, f := range list {
		names += f.Name + "|"
	}
	if err != nil || names != "Alpha mine|Ben shared|Zeta mine|" {
		t.Fatalf("Ann sees %s %v", names, err)
	}
	got, err := repo.FilterByID(ctx, id(4))
	if err != nil || got.Owner != Ben || !got.Shared || got.JQL != "type = bug" || got.Version != 1 {
		t.Fatalf("by id = %+v %v", got, err)
	}

	stale, _ := repo.FilterByID(ctx, id(1))
	_ = mine.Edit("Renamed", "project = KYB", true)
	if err := repo.SaveFilter(ctx, mine); err != nil || mine.Version != 2 {
		t.Fatalf("update: %v", err)
	}
	if err := repo.SaveFilter(ctx, stale); !errors.Is(err, domain.ErrFilterConflict) {
		t.Fatalf("stale: %v", err)
	}
	if got, _ := repo.FilterByID(ctx, id(1)); got.Name != "Renamed" || got.JQL != "project = KYB" || !got.Shared {
		t.Fatalf("updated = %+v", got)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.FilterByID(other, id(4)); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("other workspace: %v", err)
	}
	if list, _ := repo.FiltersFor(other, Ann); len(list) != 0 {
		t.Fatalf("other workspace lists %d", len(list))
	}
	if err := repo.DeleteFilter(other, id(1)); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("delete from other workspace: %v", err)
	}
	if err := repo.DeleteFilter(ctx, id(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FilterByID(ctx, id(1)); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := repo.DeleteFilter(ctx, id(1)); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := repo.FilterByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrFilterNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}
