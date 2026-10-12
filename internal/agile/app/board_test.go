package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/adapter/memory"
	"github.com/bakhod1r/kyber/internal/agile/app"
	"github.com/bakhod1r/kyber/internal/agile/domain"
)

const lead = "u-lead"

// fakeAdmin: lead administers KYB; members are forbidden; outsiders get not-found.
type fakeAdmin struct{ fakeAccess }

func (f fakeAdmin) Administer(ctx context.Context, actor, project string) error {
	if err := f.Authorize(ctx, actor, project, false); err != nil {
		return err
	}
	if actor != lead {
		return app.ErrForbidden
	}
	return nil
}

type fakeWorkflows map[string][]app.WorkflowStatus

func (f fakeWorkflows) Statuses(_ context.Context, project string) ([]app.WorkflowStatus, error) {
	if s, ok := f[project]; ok {
		return s, nil
	}
	return nil, errors.New("boom")
}

func boardSetup() *app.Service {
	n := 0
	access := fakeAccess{"KYB": {dev: "w", viewer: "r", lead: "w"}, "OPS": {dev: "w"}}
	return app.NewService(app.Deps{
		Sprints: memory.NewRepository(), Boards: memory.NewBoardRepository(), Issues: &fakeIssues{},
		Access: access, Admins: fakeAdmin{access},
		Workflows: fakeWorkflows{"KYB": {{ID: "todo", Name: "To Do"}, {ID: "review", Name: "Review"}, {ID: "done", Name: "Done"}}},
		NewID:     func() string { n++; return fmt.Sprintf("b-%d", n) },
		Now:       time.Now,
	})
}

func TestBoardsCreatesTheDefaultBoardOnFirstUse(t *testing.T) {
	ctx := context.Background()
	s := boardSetup()
	list, err := s.Boards(ctx, viewer, "KYB")
	if err != nil || len(list) != 1 {
		t.Fatalf("Boards = %v, %v", list, err)
	}
	b := list[0]
	if b.Name() != "KYB board" || b.Kind() != domain.KindKanban || len(b.Columns()) != 3 || b.Columns()[1].Name != "Review" || b.Columns()[1].Statuses[0] != "review" {
		t.Fatalf("default board = %+v %+v", b, b.Columns())
	}
	again, _ := s.Boards(ctx, dev, "KYB")
	if len(again) != 1 || again[0].ID() != b.ID() {
		t.Fatal("the default board is created once")
	}
	if _, err := s.Boards(ctx, alien, "KYB"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("alien err = %v", err)
	}
	if _, err := s.Boards(ctx, dev, "OPS"); err == nil {
		t.Fatal("workflow failure must surface")
	}
}

func TestCreateUpdateDeleteBoard(t *testing.T) {
	ctx := context.Background()
	s := boardSetup()
	spec := app.BoardSpec{Name: "Scrum", Kind: domain.KindScrum, Columns: []domain.Column{
		{Name: "Open", Statuses: []string{"todo"}}, {Name: "Doing", Statuses: []string{"review"}, Max: 2}, {Name: "Shipped", Statuses: []string{"done"}}}}
	for who, want := range map[string]error{dev: app.ErrForbidden, alien: app.ErrProjectNotFound} {
		if _, err := s.CreateBoard(ctx, who, "KYB", spec); !errors.Is(err, want) {
			t.Errorf("%s create err = %v", who, err)
		}
	}
	bad := spec
	bad.Columns = []domain.Column{{Name: "X", Statuses: []string{"ghost"}}}
	if _, err := s.CreateBoard(ctx, lead, "KYB", bad); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("unknown status err = %v", err)
	}
	b, err := s.CreateBoard(ctx, lead, "KYB", spec)
	if err != nil || b.Version() != 1 {
		t.Fatalf("create = %v, %v", b, err)
	}
	// No columns given → one column per workflow status.
	k, err := s.CreateBoard(ctx, lead, "KYB", app.BoardSpec{Name: "Kanban", Kind: domain.KindKanban})
	if err != nil || len(k.Columns()) != 3 {
		t.Fatalf("default columns = %v, %v", k, err)
	}

	got, err := s.Board(ctx, viewer, string(b.ID()))
	if err != nil || got.Name() != "Scrum" {
		t.Fatalf("Board = %v, %v", got, err)
	}
	if _, err := s.Board(ctx, alien, string(b.ID())); !errors.Is(err, domain.ErrBoardNotFound) {
		t.Fatalf("alien get err = %v", err)
	}

	spec.Name = "Team Scrum"
	if _, err := s.UpdateBoard(ctx, dev, string(b.ID()), 1, spec); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member update err = %v", err)
	}
	if _, err := s.UpdateBoard(ctx, alien, string(b.ID()), 1, spec); !errors.Is(err, domain.ErrBoardNotFound) {
		t.Fatalf("alien update err = %v", err)
	}
	if _, err := s.UpdateBoard(ctx, lead, string(b.ID()), 7, spec); !errors.Is(err, domain.ErrBoardConflict) {
		t.Fatalf("stale update err = %v", err)
	}
	if _, err := s.UpdateBoard(ctx, lead, string(b.ID()), 1, bad); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("invalid update err = %v", err)
	}
	if _, err := s.UpdateBoard(ctx, lead, string(b.ID()), 1, app.BoardSpec{Name: " ", Kind: domain.KindScrum, Columns: spec.Columns}); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("blank name err = %v", err)
	}
	up, err := s.UpdateBoard(ctx, lead, string(b.ID()), 1, spec)
	if err != nil || up.Name() != "Team Scrum" || up.Version() != 2 {
		t.Fatalf("update = %v, %v", up, err)
	}

	if err := s.DeleteBoard(ctx, dev, string(k.ID())); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member delete err = %v", err)
	}
	if err := s.DeleteBoard(ctx, lead, string(k.ID())); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBoard(ctx, lead, string(b.ID())); !errors.Is(err, app.ErrLastBoard) {
		t.Fatalf("last board err = %v", err)
	}
	if err := s.DeleteBoard(ctx, lead, "nope"); !errors.Is(err, domain.ErrBoardNotFound) {
		t.Fatalf("missing delete err = %v", err)
	}
}
