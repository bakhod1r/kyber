package repotest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// LinkFactory returns an empty link repository and three saved issues (KYB-1..3), plus the
// stored outbox event names.
type LinkFactory func(t *testing.T) (domain.LinkRepository, [3]*domain.Issue, func() []string)

func RunLinks(t *testing.T, newRepo LinkFactory) {
	ctx := context.Background()
	link := func(t *testing.T, n int, typ domain.LinkType, a, b *domain.Issue) domain.Link {
		t.Helper()
		l, err := domain.NewLink(domain.LinkID(uuidFor(100+n)), typ, a, b, domain.UserID(Assignee))
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	t.Run("add, list both ends, remove", func(t *testing.T) {
		repo, is, outbox := newRepo(t)
		l1 := link(t, 1, domain.LinkBlocks, is[0], is[1])
		l2 := link(t, 2, domain.LinkRelates, is[2], is[0])
		for _, l := range []domain.Link{l1, l2} {
			if err := repo.Add(ctx, l, []domain.Event{l.Linked()}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.ForIssue(ctx, is[0].ID())
		if err != nil || len(got) != 2 || got[0].ID != l1.ID || got[1].ID != l2.ID {
			t.Fatalf("links of KYB-1 = %+v, %v", got, err)
		}
		if got[0].FromKey != is[0].Key() || got[0].ToKey != is[1].Key() || got[0].Type != domain.LinkBlocks {
			t.Fatalf("values = %+v", got[0])
		}
		if b, _ := repo.ForIssue(ctx, is[1].ID()); len(b) != 1 {
			t.Fatalf("links of KYB-2 = %+v", b)
		}
		one, err := repo.ByID(ctx, l1.ID)
		if err != nil || one.To != is[1].ID() {
			t.Fatalf("ByID = %+v, %v", one, err)
		}
		if err := repo.Remove(ctx, l1.ID, []domain.Event{l1.Unlinked(domain.UserID(Assignee))}); err != nil {
			t.Fatal(err)
		}
		if err := repo.Remove(ctx, l1.ID, nil); !errors.Is(err, domain.ErrLinkNotFound) {
			t.Fatalf("second remove err = %v", err)
		}
		if _, err := repo.ByID(ctx, l1.ID); !errors.Is(err, domain.ErrLinkNotFound) {
			t.Fatalf("ByID after remove err = %v", err)
		}
		names := outbox()
		if len(names) < 3 || names[len(names)-1] != "issue.unlinked" {
			t.Fatalf("outbox = %v", names)
		}
	})
	t.Run("duplicates are refused, relates in either direction", func(t *testing.T) {
		repo, is, _ := newRepo(t)
		_ = repo.Add(ctx, link(t, 1, domain.LinkBlocks, is[0], is[1]), nil)
		if err := repo.Add(ctx, link(t, 2, domain.LinkBlocks, is[0], is[1]), nil); !errors.Is(err, domain.ErrLinkExists) {
			t.Fatalf("duplicate err = %v", err)
		}
		if err := repo.Add(ctx, link(t, 3, domain.LinkBlocks, is[1], is[0]), nil); err != nil {
			t.Fatalf("reverse blocks is a different link: %v", err)
		}
		_ = repo.Add(ctx, link(t, 4, domain.LinkRelates, is[0], is[2]), nil)
		if err := repo.Add(ctx, link(t, 5, domain.LinkRelates, is[2], is[0]), nil); !errors.Is(err, domain.ErrLinkExists) {
			t.Fatalf("reverse relates err = %v", err)
		}
	})
	t.Run("workspaces are isolated", func(t *testing.T) {
		repo, is, _ := newRepo(t)
		l := link(t, 1, domain.LinkBlocks, is[0], is[1])
		_ = repo.Add(ctx, l, nil)
		other := tenant.With(ctx, "00000000-0000-4000-8000-0000000000aa")
		if got, _ := repo.ForIssue(other, is[0].ID()); len(got) != 0 {
			t.Fatalf("leaked: %+v", got)
		}
		if err := repo.Remove(other, l.ID, nil); !errors.Is(err, domain.ErrLinkNotFound) {
			t.Fatalf("removed across workspaces: %v", err)
		}
	})
}
