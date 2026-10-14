package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	app "github.com/bakhod1r/kyber/internal/whiteboard/application"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/memory"
)

type convs map[string][]string

func (c convs) Members(_ context.Context, actor, conv string) ([]string, error) {
	for _, m := range c[conv] {
		if m == actor {
			return c[conv], nil
		}
	}
	return nil, app.ErrNoConversation
}

type told []string

func (t *told) Ops(_ context.Context, to []string, _ domain.WhiteboardID, by string, res []domain.OpResult) {
	*t = append(*t, fmt.Sprintf("ops %v by %s: %d", to, by, len(res)))
}
func (t *told) Cursor(_ context.Context, to []string, _ domain.WhiteboardID, by string, x, y float64) {
	*t = append(*t, fmt.Sprintf("cursor %v by %s at %v,%v", to, by, x, y))
}
func (t *told) Commented(_ context.Context, to []string, c *domain.Comment, removed bool) {
	*t = append(*t, fmt.Sprintf("comment %v on %s removed=%v", to, c.Element, removed))
}

// flaky loses the first save to another writer, to prove the reload-and-reapply.
type flaky struct {
	*memory.Repository
	lost bool
}

func (f *flaky) Save(ctx context.Context, id domain.WhiteboardID, r []domain.OpResult) error {
	if !f.lost {
		f.lost = true
		return domain.ErrStale
	}
	return f.Repository.Save(ctx, id, r)
}

func TestWhiteboard(t *testing.T) {
	tl := &told{}
	repo := &flaky{Repository: memory.NewRepository(), lost: true}
	n := 0
	svc := app.NewService(app.Deps{Boards: repo, Conversations: convs{"dev": {"lead", "dev"}}, Notifier: tl,
		NewID: func() string { n++; return fmt.Sprint("b-", n) }, Now: func() time.Time { return time.Unix(0, 0) }})
	ctx := tenant.With(context.Background(), "w1")

	b, err := svc.Create(ctx, "lead", "dev", "Architecture")
	if err != nil || b.Workspace != "w1" {
		t.Fatalf("create = %+v %v", b, err)
	}
	if _, err := svc.Create(ctx, "eve", "dev", "x"); !errors.Is(err, app.ErrNoConversation) {
		t.Fatalf("outsider create = %v", err)
	}
	if list, _ := svc.List(ctx, "dev", "dev"); len(list) != 1 {
		t.Fatalf("list = %v", list)
	}
	repo.lost = false // the next save loses a race once
	res, err := svc.Apply(ctx, "dev", string(b.ID), []domain.Op{{Kind: domain.OpAdd, Element: domain.Element{ID: "e-1", Kind: domain.KindSticky, Text: "Cache?"}}})
	if err != nil || !res[0].Applied {
		t.Fatalf("apply = %+v %v", res, err)
	}
	got, _ := svc.Get(ctx, "lead", string(b.ID))
	if got.Elements["e-1"].Text != "Cache?" {
		t.Fatalf("board = %+v", got)
	}
	if _, err := svc.Get(ctx, "eve", string(b.ID)); !errors.Is(err, domain.ErrWhiteboardMissing) {
		t.Fatalf("outsider get = %v", err)
	}
	if _, err := svc.Get(tenant.With(context.Background(), "w2"), "lead", string(b.ID)); !errors.Is(err, domain.ErrWhiteboardMissing) {
		t.Fatalf("other workspace = %v", err)
	}
	if err := svc.Cursor(ctx, "lead", string(b.ID), 10, 20); err != nil {
		t.Fatal(err)
	}
	want := "[ops [lead dev] by dev: 1 cursor [dev] by lead at 10,20]"
	if fmt.Sprint(*tl) != want {
		t.Fatalf("told = %v, want %v", *tl, want)
	}
}

// S86: comments pinned to elements, for the conversation's members; only the author removes one.
func TestElementComments(t *testing.T) {
	tl := &told{}
	n := 0
	svc := app.NewService(app.Deps{Boards: memory.NewRepository(), Comments: memory.NewComments(), Conversations: convs{"dev": {"lead", "dev"}}, Notifier: tl,
		NewID: func() string { n++; return fmt.Sprint("x-", n) }, Now: func() time.Time { return time.Unix(0, 0) }})
	ctx := tenant.With(context.Background(), "w1")
	b, _ := svc.Create(ctx, "lead", "dev", "Architecture")
	*tl = nil
	c, err := svc.Comment(ctx, "dev", string(b.ID), "e-1", "Is this cached?")
	if err != nil || len(*tl) != 1 || (*tl)[0] != "comment [lead dev] on e-1 removed=false" {
		t.Fatalf("comment = %+v %v %v", c, err, *tl)
	}
	if _, err := svc.Comment(ctx, "eve", string(b.ID), "e-1", "x"); !errors.Is(err, domain.ErrWhiteboardMissing) {
		t.Fatalf("outsider = %v", err)
	}
	if list, _ := svc.Comments(ctx, "lead", string(b.ID)); len(list) != 1 || list[0].Body != "Is this cached?" {
		t.Fatalf("comments = %+v", list)
	}
	if err := svc.RemoveComment(ctx, "lead", string(c.ID)); !errors.Is(err, app.ErrNotAuthor) {
		t.Fatalf("someone else removes = %v", err)
	}
	if err := svc.RemoveComment(ctx, "dev", string(c.ID)); err != nil {
		t.Fatal(err)
	}
}
