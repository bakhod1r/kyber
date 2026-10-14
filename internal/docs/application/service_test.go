package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/application"
	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/docs/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

const (
	lead   = "u-lead"
	viewer = "u-viewer"
	out    = "u-outsider"
	broken = "u-broken" // the project check fails
)

var errDown = errors.New("db down")

type projects struct{}

func (projects) Authorize(_ context.Context, actor, project string, change bool) error {
	switch {
	case actor == broken:
		return errDown
	case project != "KYB" || actor == out:
		return application.ErrProjectNotFound
	case change && actor == viewer:
		return application.ErrForbidden
	}
	return nil
}

func TestPages(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.FixedZone("UZ", 5*3600))
	n := 0
	s := application.NewService(application.Deps{Projects: projects{}, Pages: memory.NewRepository(),
		NewID: func() string { n++; return fmt.Sprintf("p-%d", n) }, Now: func() time.Time { return now }})

	p, err := s.Create(ctx, lead, "KYB", "Guide", "# Hi")
	if err != nil || p.Version != 1 || p.UpdatedAt.Location() != time.UTC {
		t.Fatalf("create = %+v %v", p, err)
	}
	for _, tc := range []struct {
		actor, project, title string
		want                  error
	}{
		{viewer, "KYB", "x", application.ErrForbidden},
		{out, "KYB", "x", application.ErrProjectNotFound},
		{lead, "KYB", "", domain.ErrInvalidPage},
	} {
		if _, err := s.Create(ctx, tc.actor, tc.project, tc.title, ""); !errors.Is(err, tc.want) {
			t.Errorf("create %+v: %v", tc, err)
		}
	}
	if list, err := s.Pages(ctx, viewer, "KYB"); err != nil || len(list) != 1 {
		t.Fatalf("pages = %v %v", list, err)
	}
	if _, err := s.Pages(ctx, out, "KYB"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider lists: %v", err)
	}
	id := string(p.ID)
	if got, err := s.Page(ctx, viewer, id); err != nil || got.Body != "# Hi" {
		t.Fatalf("page = %+v %v", got, err)
	}
	if _, err := s.Page(ctx, out, id); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("outsider reads: %v", err)
	}
	if _, err := s.Page(ctx, lead, "nope"); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("missing page: %v", err)
	}

	now = now.Add(time.Hour)
	p, err = s.Revise(ctx, lead, id, 1, "Guide", "# Hello")
	if err != nil || p.Version != 2 {
		t.Fatalf("revise = %+v %v", p, err)
	}
	if same, err := s.Revise(ctx, lead, id, 2, "Guide", "# Hello"); err != nil || same.Version != 2 {
		t.Fatalf("unchanged save = %+v %v", same, err)
	}
	for _, tc := range []struct {
		actor   string
		version int
		title   string
		want    error
	}{
		{lead, 1, "x", domain.ErrPageConflict},
		{lead, 2, "", domain.ErrInvalidPage},
		{viewer, 2, "x", application.ErrForbidden},
	} {
		if _, err := s.Revise(ctx, tc.actor, id, tc.version, tc.title, ""); !errors.Is(err, tc.want) {
			t.Errorf("revise %+v: %v", tc, err)
		}
	}
	hist, err := s.History(ctx, viewer, id)
	if err != nil || len(hist) != 2 || hist[0].Body != "# Hello" {
		t.Fatalf("history = %+v %v", hist, err)
	}
	if _, err := s.History(ctx, out, id); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("outsider history: %v", err)
	}
	if err := s.Delete(ctx, viewer, id); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer deletes: %v", err)
	}
	if err := s.Delete(ctx, lead, id); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, lead, id); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

// S86: anyone who may read a page may comment on its paragraphs; only the author removes a comment.
func TestParagraphComments(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	n := 0
	pages := memory.NewRepository()
	comments := memory.NewComments(pages)
	s := application.NewService(application.Deps{Projects: projects{}, Pages: pages, Comments: comments,
		NewID: func() string { n++; return fmt.Sprintf("x-%d", n) }, Now: time.Now})
	p, _ := s.Create(ctx, lead, "KYB", "Guide", "a\n\nb")
	id := string(p.ID)

	c, err := s.Comment(ctx, viewer, id, 1, "b", "Why? @lead@x.uz")
	if err != nil || c.Author != viewer || c.Paragraph != 1 {
		t.Fatalf("comment = %+v %v", c, err)
	}
	if ev := comments.Outbox(); len(ev) != 1 || ev[0].Workspace != tenant.Default || ev[0].Title != "Guide" {
		t.Fatalf("outbox = %+v", ev)
	}
	if _, err := s.Comment(ctx, out, id, 0, "", "x"); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("outsider comments: %v", err)
	}
	if _, err := s.Comment(ctx, lead, id, -1, "", "x"); !errors.Is(err, domain.ErrInvalidComment) {
		t.Fatalf("bad paragraph: %v", err)
	}
	if list, err := s.Comments(ctx, lead, id); err != nil || len(list) != 1 {
		t.Fatalf("comments = %v %v", list, err)
	}
	if _, err := s.Comments(ctx, out, id); !errors.Is(err, domain.ErrPageNotFound) {
		t.Fatalf("outsider lists: %v", err)
	}
	if err := s.RemoveComment(ctx, lead, string(c.ID)); !errors.Is(err, application.ErrNotAuthor) {
		t.Fatalf("someone else removes: %v", err)
	}
	if err := s.RemoveComment(ctx, out, string(c.ID)); !errors.Is(err, domain.ErrCommentMissing) {
		t.Fatalf("outsider removes: %v", err)
	}
	if err := s.RemoveComment(ctx, broken, string(c.ID)); !errors.Is(err, errDown) {
		t.Fatalf("failing check: %v", err)
	}
	if err := s.RemoveComment(ctx, viewer, "nope"); !errors.Is(err, domain.ErrCommentMissing) {
		t.Fatalf("missing: %v", err)
	}
	if err := s.RemoveComment(ctx, viewer, string(c.ID)); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Comments(ctx, lead, id); len(list) != 0 {
		t.Fatalf("after remove: %v", list)
	}
}
