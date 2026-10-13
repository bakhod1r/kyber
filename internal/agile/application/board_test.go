package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/application"
	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/agile/infrastructure/memory"
)

const lead = "u-lead"

// fakeAdmin: lead administers KYB; members are forbidden; outsiders get not-found.
type fakeAdmin struct{ fakeAccess }

func (f fakeAdmin) Administer(ctx context.Context, actor, project string) error {
	if err := f.Authorize(ctx, actor, project, false); err != nil {
		return err
	}
	if actor != lead {
		return application.ErrForbidden
	}
	return nil
}

type fakeWorkflows map[string][]application.WorkflowStatus

func (f fakeWorkflows) Statuses(_ context.Context, project string) ([]application.WorkflowStatus, error) {
	if s, ok := f[project]; ok {
		return s, nil
	}
	return nil, errors.New("boom")
}

func boardSetup() *application.Service {
	n := 0
	access := fakeAccess{"KYB": {dev: "w", viewer: "r", lead: "w"}, "OPS": {dev: "w"}}
	return application.NewService(application.Deps{
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
	if _, err := s.Boards(ctx, alien, "KYB"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("alien err = %v", err)
	}
	if _, err := s.Boards(ctx, dev, "OPS"); err == nil {
		t.Fatal("workflow failure must surface")
	}
}

func TestCreateUpdateDeleteBoard(t *testing.T) {
	ctx := context.Background()
	s := boardSetup()
	spec := application.BoardSpec{Name: "Scrum", Kind: domain.KindScrum, Columns: []domain.Column{
		{Name: "Open", Statuses: []string{"todo"}}, {Name: "Doing", Statuses: []string{"review"}, Max: 2}, {Name: "Shipped", Statuses: []string{"done"}}}}
	for who, want := range map[string]error{dev: application.ErrForbidden, alien: application.ErrProjectNotFound} {
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
	k, err := s.CreateBoard(ctx, lead, "KYB", application.BoardSpec{Name: "Kanban", Kind: domain.KindKanban})
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
	if _, err := s.UpdateBoard(ctx, dev, string(b.ID()), 1, spec); !errors.Is(err, application.ErrForbidden) {
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
	if _, err := s.UpdateBoard(ctx, lead, string(b.ID()), 1, application.BoardSpec{Name: " ", Kind: domain.KindScrum, Columns: spec.Columns}); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("blank name err = %v", err)
	}
	up, err := s.UpdateBoard(ctx, lead, string(b.ID()), 1, spec)
	if err != nil || up.Name() != "Team Scrum" || up.Version() != 2 {
		t.Fatalf("update = %v, %v", up, err)
	}

	if err := s.DeleteBoard(ctx, dev, string(k.ID())); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member delete err = %v", err)
	}
	if err := s.DeleteBoard(ctx, lead, string(k.ID())); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBoard(ctx, lead, string(b.ID())); !errors.Is(err, application.ErrLastBoard) {
		t.Fatalf("last board err = %v", err)
	}
	if err := s.DeleteBoard(ctx, lead, "nope"); !errors.Is(err, domain.ErrBoardNotFound) {
		t.Fatalf("missing delete err = %v", err)
	}
}

// The auto-created board follows the workflow: a status added later becomes a column (S43 + S44).
func TestDefaultBoardFollowsWorkflow(t *testing.T) {
	ctx := context.Background()
	wfs := fakeWorkflows{"KYB": {{ID: "todo", Name: "To Do"}, {ID: "done", Name: "Done"}}}
	access := fakeAccess{"KYB": {dev: "w", lead: "w"}}
	s := application.NewService(application.Deps{Sprints: memory.NewRepository(), Boards: memory.NewBoardRepository(), Issues: &fakeIssues{},
		Access: access, Admins: fakeAdmin{access}, Workflows: wfs, NewID: func() string { return "b-1" }, Now: time.Now})
	list, _ := s.Boards(ctx, dev, "KYB")
	if len(list[0].Columns()) != 2 || !list[0].FollowsWorkflow() {
		t.Fatalf("default = %+v", list[0].Columns())
	}
	wfs["KYB"] = []application.WorkflowStatus{{ID: "todo", Name: "To Do"}, {ID: "review", Name: "Code Review"}, {ID: "done", Name: "Done"}}
	list, _ = s.Boards(ctx, dev, "KYB")
	b, _ := s.Board(ctx, dev, "b-1")
	if len(list[0].Columns()) != 3 || list[0].Columns()[1].Name != "Code Review" || len(b.Columns()) != 3 {
		t.Fatalf("after workflow change = %+v", list[0].Columns())
	}
}

func TestKanbanBacklogBoards(t *testing.T) {
	ctx := context.Background()
	s := boardSetup()
	b, err := s.CreateBoard(ctx, lead, "KYB", application.BoardSpec{Name: "Flow", Kind: domain.KindKanban, KanbanBacklog: true})
	if err != nil || !b.KanbanBacklog() || b.BacklogStatuses()[0] != "todo" {
		t.Fatalf("create = %v %v", b, err)
	}
	if _, err := s.CreateBoard(ctx, lead, "KYB", application.BoardSpec{Name: "Sprints", Kind: domain.KindScrum, KanbanBacklog: true}); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("scrum with kanban backlog: %v", err)
	}
	u, err := s.UpdateBoard(ctx, lead, string(b.ID()), b.Version(), application.BoardSpec{Name: "Flow", Kind: domain.KindKanban})
	if err != nil || u.KanbanBacklog() {
		t.Fatalf("turn off: %v %v", u, err)
	}
	if _, err := s.UpdateBoard(ctx, lead, string(b.ID()), u.Version(), application.BoardSpec{Name: "Flow", Kind: domain.KindScrum, KanbanBacklog: true}); !errors.Is(err, domain.ErrInvalidBoard) {
		t.Fatalf("update to scrum with backlog: %v", err)
	}
}
