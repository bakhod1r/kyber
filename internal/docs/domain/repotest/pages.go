// Package repotest is the contract every page repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Users and workspace the factory must create.
const (
	Ann            = "81000000-0000-4000-8000-000000000001"
	Ben            = "81000000-0000-4000-8000-000000000002"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ac"
)

func id(n int) domain.PageID {
	return domain.PageID("82000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, project, title string) *domain.Page {
		p, rev, err := domain.NewPage(id(n), project, title, "# "+title, Ann, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, p, rev); err != nil || p.Version != 1 {
			t.Fatalf("save %s: %v", title, err)
		}
		return p
	}
	guide := mk(1, "KYB", "Guide")
	mk(2, "KYB", "Architecture")
	mk(3, "OPS", "Runbook")

	list, err := repo.ForProject(ctx, "KYB")
	if err != nil || len(list) != 2 || list[0].Title != "Architecture" || list[1].Title != "Guide" {
		t.Fatalf("for project = %v", err)
	}
	got, err := repo.ByID(ctx, guide.ID)
	if err != nil || got.Title != "Guide" || got.Body != "# Guide" || got.Author != Ann || !got.UpdatedAt.Equal(t0) || got.Version != 1 {
		t.Fatalf("by id = %+v %v", got, err)
	}

	rev, _, _ := got.Revise("Guide v2", "## Two", Ben, t0.Add(time.Hour))
	if err := repo.Save(ctx, got, rev); err != nil || got.Version != 2 {
		t.Fatalf("revise: %v %d", err, got.Version)
	}
	again, _ := repo.ByID(ctx, guide.ID)
	if again.Title != "Guide v2" || again.UpdatedBy != Ben || again.Author != Ann || !again.UpdatedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("revised = %+v", again)
	}
	hist, err := repo.History(ctx, guide.ID)
	if err != nil || len(hist) != 2 || hist[0].Number != 2 || hist[0].By != Ben || hist[1].Title != "Guide" || !hist[1].At.Equal(t0) {
		t.Fatalf("history = %+v %v", hist, err)
	}

	stale := *guide // version 1
	rev2, _, _ := stale.Revise("Stale", "x", Ann, t0)
	if err := repo.Save(ctx, &stale, rev2); !errors.Is(err, domain.ErrPageConflict) {
		t.Fatalf("stale save: %v", err)
	}
	dup, drev, _ := domain.NewPage(guide.ID, "KYB", "Dup", "", Ann, t0)
	if err := repo.Save(ctx, dup, drev); !errors.Is(err, domain.ErrPageConflict) {
		t.Fatalf("duplicate insert: %v", err)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.ByID(other, guide.ID); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("other workspace reads: %v", err)
	}
	if h, _ := repo.History(other, guide.ID); len(h) != 0 {
		t.Fatalf("other workspace history: %v", h)
	}
	if l, _ := repo.ForProject(other, "KYB"); len(l) != 0 {
		t.Fatalf("other workspace lists: %v", l)
	}
	if err := repo.Delete(other, guide.ID); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("other workspace deletes: %v", err)
	}
	for _, bad := range []domain.PageID{"not-an-id"} {
		if _, err := repo.ByID(ctx, bad); !errors.Is(err, domain.ErrPageNotFound) {
			t.Fatalf("bad id: %v", err)
		}
		if h, err := repo.History(ctx, bad); err != nil || len(h) != 0 {
			t.Fatalf("bad id history: %v %v", h, err)
		}
		if err := repo.Delete(ctx, bad); !errors.Is(err, domain.ErrPageNotFound) {
			t.Fatalf("bad id delete: %v", err)
		}
	}
	if err := repo.Delete(ctx, guide.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByID(ctx, guide.ID); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if h, _ := repo.History(ctx, guide.ID); len(h) != 0 {
		t.Fatalf("history outlives the page: %v", h)
	}
}
