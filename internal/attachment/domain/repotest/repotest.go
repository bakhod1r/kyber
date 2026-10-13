// Package repotest is the contract every attachment repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Users and workspace the factory must create.
const (
	Ann            = "91000000-0000-4000-8000-000000000001"
	OtherWorkspace = "00000000-0000-4000-8000-0000000000ad"
)

func id(n int) domain.ID {
	return domain.ID("92000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func Run(t *testing.T, repo domain.Repository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	mk := func(n int, issue, name string, at time.Time) *domain.Attachment {
		a, err := domain.New(id(n), issue, name, "text/plain", int64(n), "abc", Ann, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, a); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return a
	}
	first := mk(1, "KYB-1", "a.txt", t0)
	mk(2, "KYB-1", "b.txt", t0.Add(time.Minute))
	mk(3, "KYB-2", "c.txt", t0)

	got, err := repo.ByID(ctx, first.ID)
	if err != nil || *got != *first {
		t.Fatalf("by id = %+v, %v", got, err)
	}
	list, err := repo.ForIssue(ctx, "KYB-1")
	if err != nil || len(list) != 2 || list[0].Name != "a.txt" || list[1].Name != "b.txt" {
		t.Fatalf("for issue = %v, %v", list, err)
	}
	if list, _ := repo.ForIssue(ctx, "KYB-9"); len(list) != 0 {
		t.Fatalf("empty issue = %v", list)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.ByID(other, first.ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("other workspace by id: %v", err)
	}
	if list, _ := repo.ForIssue(other, "KYB-1"); len(list) != 0 {
		t.Fatalf("other workspace sees %v", list)
	}
	if err := repo.Delete(other, first.ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("other workspace delete: %v", err)
	}

	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByID(ctx, first.ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := repo.Delete(ctx, first.ID); !errors.Is(err, domain.ErrAttachmentNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}
