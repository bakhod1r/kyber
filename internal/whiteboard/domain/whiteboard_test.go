package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func rect(id string) domain.Element {
	return domain.Element{ID: domain.ElementID(id), Kind: domain.KindRect, X: 10, Y: 20, W: 100, H: 50, Color: "#0b6fd8"}
}

func TestNewWhiteboard(t *testing.T) {
	b, err := domain.New("b-1", "w-1", "c-1", " Architecture ", "u-a", t0)
	if err != nil || b.Title != "Architecture" || len(b.Elements) != 0 {
		t.Fatalf("board = %+v %v", b, err)
	}
	if _, err := domain.New("b", "w", "c", " ", "u", t0); !errors.Is(err, domain.ErrInvalidTitle) {
		t.Fatalf("blank title = %v", err)
	}
}

func TestOps(t *testing.T) {
	b, _ := domain.New("b-1", "w-1", "c-1", "Arch", "u-a", t0)
	res := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: rect("e-1")}}, t0)
	if len(res) != 1 || !res[0].Applied || res[0].Element.Version != 1 || res[0].Element.UpdatedBy != "u-a" {
		t.Fatalf("add = %+v", res)
	}
	// Two people move the same shape from version 1: the first wins, the second is told the current state.
	moveA := rect("e-1")
	moveA.X, moveA.Version = 200, 1
	moveB := rect("e-1")
	moveB.X, moveB.Version = 300, 1
	r1 := b.Apply("u-a", []domain.Op{{Kind: domain.OpUpdate, Element: moveA}}, t0)
	r2 := b.Apply("u-b", []domain.Op{{Kind: domain.OpUpdate, Element: moveB}}, t0)
	if !r1[0].Applied || r1[0].Element.Version != 2 || r2[0].Applied || r2[0].Element.X != 200 || b.Elements["e-1"].X != 200 {
		t.Fatalf("concurrent moves = %+v / %+v", r1, r2)
	}
	if r := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: rect("e-1")}}, t0); r[0].Applied || r[0].Error == nil {
		t.Fatalf("add twice = %+v", r)
	}
	bad := rect("e-2")
	bad.Kind = "triangle"
	long := domain.Element{ID: "e-3", Kind: domain.KindText, Text: string(make([]byte, domain.MaxText+1))}
	thick := rect("e-4")
	thick.Stroke = domain.StrokeBold + 1 // S91: thin, medium or bold only
	thin := rect("e-5")
	thin.Stroke = -1
	for _, e := range []domain.Element{bad, long, {Kind: domain.KindRect}, thick, thin} {
		if r := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: e}}, t0); r[0].Applied || !errors.Is(r[0].Error, domain.ErrInvalidElement) {
			t.Errorf("invalid %+v = %+v", e.Kind, r)
		}
	}
	styled := rect("e-6")
	styled.Stroke, styled.Fill, styled.Dashed = domain.StrokeBold, true, true
	if r := b.Apply("u-a", []domain.Op{{Kind: domain.OpAdd, Element: styled}}, t0); !r[0].Applied || !r[0].Element.Fill || r[0].Element.Stroke != domain.StrokeBold {
		t.Fatalf("styled = %+v", r)
	}
	delete(b.Elements, "e-6")
	del := domain.Element{ID: "e-1", Version: 2}
	if r := b.Apply("u-b", []domain.Op{{Kind: domain.OpDelete, Element: del}}, t0); !r[0].Applied || len(b.Elements) != 0 || !r[0].Element.Deleted {
		t.Fatalf("delete = %+v", r)
	}
	if r := b.Apply("u-b", []domain.Op{{Kind: domain.OpUpdate, Element: moveA}}, t0); r[0].Applied || !errors.Is(r[0].Error, domain.ErrElementNotFound) {
		t.Fatalf("update deleted = %+v", r)
	}
}

func TestBoardIsCapped(t *testing.T) {
	b, _ := domain.New("b-1", "w-1", "c-1", "Arch", "u-a", t0)
	ops := make([]domain.Op, domain.MaxElements+1)
	for i := range ops {
		ops[i] = domain.Op{Kind: domain.OpAdd, Element: rect(string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676)))}
	}
	res := b.Apply("u-a", ops, t0)
	if !res[domain.MaxElements-1].Applied || res[domain.MaxElements].Applied || !errors.Is(res[domain.MaxElements].Error, domain.ErrBoardFull) {
		t.Fatalf("cap: last ok = %v, over = %+v", res[domain.MaxElements-1].Applied, res[domain.MaxElements])
	}
}
