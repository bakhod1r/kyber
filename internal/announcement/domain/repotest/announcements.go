// Package repotest is the contract every announcement repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Author and workspace the factory must create.
const (
	Author         = "a1000000-0000-4000-8000-000000000001"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ae"
)

func id(n int) domain.AnnouncementID {
	return domain.AnnouncementID("a2000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, project, title string, at time.Time) *domain.Announcement {
		a, err := domain.Publish(id(n), tenant.Default, project, title, "Body "+title, Author, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("save %s: %v", title, err)
		}
		return a
	}
	old := mk(1, "KYB", "Old", t0)
	mk(2, "KYB", "New", t0.Add(time.Hour))
	mk(3, "", "Holiday", t0)
	mk(4, "OPS", "Elsewhere", t0)

	list, err := repo.For(ctx, "KYB", 10)
	if err != nil || len(list) != 2 || list[0].Title != "New" || list[1].Title != "Old" {
		t.Fatalf("project list = %v", err)
	}
	if list, _ := repo.For(ctx, "KYB", 1); len(list) != 1 {
		t.Fatalf("limit ignored: %d", len(list))
	}
	if ws, _ := repo.For(ctx, "", 10); len(ws) != 1 || ws[0].Title != "Holiday" {
		t.Fatalf("workspace list = %v", ws)
	}
	got, err := repo.ByID(ctx, old.ID)
	if err != nil || got.Title != "Old" || got.Body != "Body Old" || got.Author != Author || !got.At.Equal(t0) || got.Project != "KYB" || got.Workspace != tenant.Default {
		t.Fatalf("by id = %+v %v", got, err)
	}
	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.ByID(other, old.ID); !errors.Is(err, domain.ErrAnnouncementNotFound) {
		t.Fatalf("other workspace reads: %v", err)
	}
	if l, _ := repo.For(other, "KYB", 10); len(l) != 0 {
		t.Fatalf("other workspace lists %d", len(l))
	}
	if _, err := repo.ByID(ctx, "not-an-id"); !errors.Is(err, domain.ErrAnnouncementNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}
