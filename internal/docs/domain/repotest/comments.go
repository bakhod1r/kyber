package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func commentID(n int) domain.CommentID {
	return domain.CommentID("83000000-0000-4000-8000-00000000000" + string(rune('0'+n)))
}

// RunComments is the contract for paragraph comments (S86); pages stores the pages they are on.
func RunComments(t *testing.T, pages domain.Repository, comments domain.Comments) {
	ctx := tenant.With(context.Background(), tenant.Default)
	p, rev, _ := domain.NewPage(id(7), "KYB", "Notes", "a\n\nb", Ann, t0)
	if err := pages.Save(ctx, p, rev); err != nil {
		t.Fatal(err)
	}
	add := func(n, para int, by, body string) *domain.Comment {
		c, err := p.Comment(commentID(n), tenant.Default, para, "b", by, body, t0.Add(time.Duration(n)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := comments.Add(ctx, c); err != nil {
			t.Fatalf("add: %v", err)
		}
		return c
	}
	second := add(2, 1, Ben, "later")
	add(1, 0, Ann, "first")

	list, err := comments.OnPage(ctx, p.ID)
	if err != nil || len(list) != 2 || list[0].Body != "first" || list[1].ID != second.ID {
		t.Fatalf("on page = %+v %v", list, err)
	}
	got, err := comments.ByID(ctx, second.ID)
	if err != nil || got.Page != p.ID || got.Project != "KYB" || got.Paragraph != 1 || got.Quote != "b" || got.Author != Ben ||
		!got.CreatedAt.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("by id = %+v %v", got, err)
	}

	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := comments.ByID(other, second.ID); !errors.Is(err, domain.ErrCommentMissing) {
		t.Fatalf("other workspace reads: %v", err)
	}
	if l, _ := comments.OnPage(other, p.ID); len(l) != 0 {
		t.Fatalf("other workspace lists: %v", l)
	}
	for _, bad := range []domain.CommentID{"not-an-id", commentID(9)} {
		if _, err := comments.ByID(ctx, bad); !errors.Is(err, domain.ErrCommentMissing) {
			t.Fatalf("missing: %v", err)
		}
		if err := comments.Remove(ctx, bad); err != nil {
			t.Fatalf("remove missing: %v", err)
		}
	}
	if l, err := comments.OnPage(ctx, "not-an-id"); err != nil || len(l) != 0 {
		t.Fatalf("bad page: %v %v", l, err)
	}

	if err := comments.Remove(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := comments.OnPage(ctx, p.ID); len(l) != 1 {
		t.Fatalf("after remove: %v", l)
	}
	if err := pages.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := comments.OnPage(ctx, p.ID); len(l) != 0 {
		t.Fatalf("comments outlive the page: %v", l)
	}
}
