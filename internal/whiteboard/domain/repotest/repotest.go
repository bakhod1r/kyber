// Package repotest is the contract every Whiteboard repository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

const (
	WS   = "00000000-0000-4000-8000-000000000001"
	Conv = "72000000-0000-4000-8000-000000000001"
	Lead = "70000000-0000-4000-8000-000000000001"
	Dev  = "70000000-0000-4000-8000-000000000002"
)

func Run(t *testing.T, r domain.Repository) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	b, _ := domain.New("75000000-0000-4000-8000-000000000001", WS, Conv, "Architecture", Lead, t0)
	other, _ := domain.New("75000000-0000-4000-8000-000000000002", WS, Conv, "Retro", Dev, t0.Add(time.Hour))
	for _, x := range []*domain.Whiteboard{b, other} {
		if err := r.Create(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.InConversation(ctx, Conv)
	if err != nil || len(list) != 2 || list[0].Title != "Retro" || list[1].CreatedBy != Lead {
		t.Fatalf("list = %v %v (newest first)", list, err)
	}

	box := domain.Element{ID: "e-1", Kind: domain.KindRect, X: 1.5, Y: 2, W: 100, H: 40, Text: "API", Color: "#112233",
		Stroke: domain.StrokeBold, Fill: true, Dashed: true}
	line := domain.Element{ID: "e-2", Kind: domain.KindArrow, X: 0, Y: 0, Points: []domain.Point{{X: 0, Y: 0}, {X: 50, Y: 25.5}}}
	res := b.Apply(Lead, []domain.Op{{Kind: domain.OpAdd, Element: box}, {Kind: domain.OpAdd, Element: line}}, t0)
	if err := r.Save(ctx, b.ID, res); err != nil {
		t.Fatal(err)
	}
	got, err := r.ByID(ctx, b.ID)
	if err != nil || got.Title != "Architecture" || got.Conversation != Conv || got.Workspace != WS || len(got.Elements) != 2 {
		t.Fatalf("ByID = %+v %v", got, err)
	}
	e1 := got.Elements["e-1"]
	if e1.X != 1.5 || e1.Text != "API" || e1.Color != "#112233" || e1.Stroke != domain.StrokeBold || !e1.Fill || !e1.Dashed || e1.Version != 1 || e1.UpdatedBy != Lead || !e1.UpdatedAt.Equal(t0) {
		t.Fatalf("e-1 = %+v", e1)
	}
	if e2 := got.Elements["e-2"]; len(e2.Points) != 2 || e2.Points[1] != (domain.Point{X: 50, Y: 25.5}) {
		t.Fatalf("e-2 = %+v", e2)
	}

	// Two writers apply against the same stored version: the second save is refused, nothing of it is kept.
	a, _ := r.ByID(ctx, b.ID)
	c, _ := r.ByID(ctx, b.ID)
	moveA, moveC := a.Elements["e-1"], c.Elements["e-1"]
	moveA.X, moveC.X = 200, 300
	ra := a.Apply(Lead, []domain.Op{{Kind: domain.OpUpdate, Element: moveA}}, t0)
	rc := c.Apply(Dev, []domain.Op{{Kind: domain.OpDelete, Element: c.Elements["e-2"]}, {Kind: domain.OpUpdate, Element: moveC}}, t0)
	if err := r.Save(ctx, b.ID, ra); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, b.ID, rc); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("stale save = %v", err)
	}
	got, _ = r.ByID(ctx, b.ID)
	if got.Elements["e-1"].X != 200 || got.Elements["e-1"].Version != 2 || len(got.Elements) != 2 {
		t.Fatalf("after race = %+v (the refused batch kept nothing, not even its delete)", got.Elements)
	}
	rd := got.Apply(Dev, []domain.Op{{Kind: domain.OpDelete, Element: got.Elements["e-2"]}}, t0)
	if err := r.Save(ctx, b.ID, rd); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.ByID(ctx, b.ID); len(got.Elements) != 1 {
		t.Fatalf("after delete = %+v", got.Elements)
	}
	for _, id := range []domain.WhiteboardID{"75000000-0000-4000-8000-000000000009", "nope"} {
		if _, err := r.ByID(ctx, id); !errors.Is(err, domain.ErrWhiteboardMissing) {
			t.Fatalf("missing %s = %v", id, err)
		}
	}
}

// RunComments checks element comments (S86) over the store of r.
func RunComments(t *testing.T, r domain.Repository, comments domain.Comments) {
	ctx := context.Background()
	t0 := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	b, _ := domain.New("75000000-0000-4000-8000-000000000091", WS, Conv, "Comments", Lead, t0)
	if err := r.Create(ctx, b); err != nil {
		t.Fatal(err)
	}
	first, _ := b.Comment("7c000000-0000-4000-8000-000000000001", "e-1", Dev, "Cache this?", t0)
	second, _ := b.Comment("7c000000-0000-4000-8000-000000000002", "e-2", Lead, "Yes", t0.Add(time.Minute))
	for _, c := range []*domain.Comment{first, second} {
		if err := comments.Add(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	list, err := comments.OnBoard(ctx, b.ID)
	if err != nil || len(list) != 2 || list[0].Body != "Cache this?" || list[0].Element != "e-1" || list[1].Author != Lead ||
		!list[1].CreatedAt.Equal(t0.Add(time.Minute)) || list[0].Conversation != Conv || list[0].Workspace != WS {
		t.Fatalf("OnBoard = %+v %v", list, err)
	}
	if err := comments.Remove(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.CommentID{first.ID, "nope"} {
		if _, err := comments.ByID(ctx, id); !errors.Is(err, domain.ErrCommentMissing) {
			t.Fatalf("ByID %s = %v", id, err)
		}
	}
}
