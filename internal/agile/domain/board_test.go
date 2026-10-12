package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

func cols() []domain.Column {
	return []domain.Column{
		{Name: "To Do", Statuses: []string{"todo"}},
		{Name: "Doing", Statuses: []string{"in_progress", "review"}, Min: 1, Max: 3},
		{Name: "Done", Statuses: []string{"done"}},
	}
}

func TestNewBoard(t *testing.T) {
	b, err := domain.NewBoard("b-1", "KYB", " Team board ", domain.KindKanban, cols())
	if err != nil {
		t.Fatal(err)
	}
	if b.ID() != "b-1" || b.Project() != "KYB" || b.Name() != "Team board" || b.Kind() != domain.KindKanban || len(b.Columns()) != 3 {
		t.Fatalf("board = %+v", b)
	}
	if b.ColumnOf("review") != 1 || b.ColumnOf("todo") != 0 || b.ColumnOf("ghost") != -1 {
		t.Fatal("ColumnOf")
	}
	b.Columns()[0].Name = "mutated"
	if b.Columns()[0].Name != "To Do" {
		t.Fatal("Columns must return a copy")
	}
}

func TestBoardValidation(t *testing.T) {
	dup := cols()
	dup[2].Statuses = []string{"done", "todo"}
	cases := []struct {
		name string
		bn   string
		kind domain.BoardKind
		cols []domain.Column
	}{
		{"empty name", " ", domain.KindScrum, cols()},
		{"long name", strings.Repeat("x", 61), domain.KindScrum, cols()},
		{"bad kind", "B", "waterfall", cols()},
		{"no columns", "B", domain.KindScrum, nil},
		{"empty column name", "B", domain.KindScrum, []domain.Column{{Name: " "}}},
		{"duplicate column", "B", domain.KindScrum, []domain.Column{{Name: "A"}, {Name: "a"}}},
		{"status in two columns", "B", domain.KindScrum, dup},
		{"negative limit", "B", domain.KindScrum, []domain.Column{{Name: "A", Min: -1}}},
		{"min above max", "B", domain.KindScrum, []domain.Column{{Name: "A", Min: 4, Max: 2}}},
		{"empty status", "B", domain.KindScrum, []domain.Column{{Name: "A", Statuses: []string{""}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := domain.NewBoard("b", "KYB", c.bn, c.kind, c.cols); !errors.Is(err, domain.ErrInvalidBoard) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestBoardConfigure(t *testing.T) {
	b, _ := domain.NewBoard("b-1", "KYB", "Board", domain.KindScrum, cols())
	if err := b.Configure("Renamed", domain.KindKanban, cols()[:1]); err != nil {
		t.Fatal(err)
	}
	if b.Name() != "Renamed" || b.Kind() != domain.KindKanban || len(b.Columns()) != 1 {
		t.Fatalf("configured = %+v", b)
	}
	if err := b.Configure("", domain.KindKanban, cols()); !errors.Is(err, domain.ErrInvalidBoard) || b.Name() != "Renamed" {
		t.Fatalf("invalid configure must not change the board: %v %s", err, b.Name())
	}
}

// Jira WIP limits: a column below its minimum or above its maximum is flagged.
func TestColumnWIP(t *testing.T) {
	c := domain.Column{Name: "Doing", Min: 1, Max: 3}
	for n, want := range map[int]domain.WIP{0: domain.WIPUnder, 1: domain.WIPOK, 3: domain.WIPOK, 4: domain.WIPOver} {
		if got := c.WIP(n); got != want {
			t.Errorf("WIP(%d) = %s, want %s", n, got, want)
		}
	}
	if (domain.Column{Name: "Free"}).WIP(100) != domain.WIPOK {
		t.Fatal("no limits means OK")
	}
}

func TestRehydrateBoard(t *testing.T) {
	b := domain.RehydrateBoard("b-1", "KYB", "Board", domain.KindScrum, cols(), 4)
	b.MarkPersisted()
	if b.Version() != 5 || b.Name() != "Board" || len(b.Columns()) != 3 {
		t.Fatalf("rehydrated = %+v", b)
	}
}
