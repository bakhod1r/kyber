package repotest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Board IDs adapters must accept (UUIDs for Postgres).
const (
	BoardA = "80000000-0000-4000-8000-000000000001"
	BoardB = "80000000-0000-4000-8000-000000000002"
	BoardC = "80000000-0000-4000-8000-000000000003"
)

// OtherWorkspace is a second workspace the factory must create (with its own KYB project).
const OtherWorkspace = "00000000-0000-4000-8000-0000000000aa"

// BoardFactory returns an empty board repository; projects KYB and OPS exist in the default workspace.
type BoardFactory func(t *testing.T) domain.BoardRepository

func board(t *testing.T, id, project, name string) *domain.Board {
	t.Helper()
	b, err := domain.NewBoard(domain.BoardID(id), project, name, domain.KindKanban, []domain.Column{
		{Name: "To Do", Statuses: []string{"todo"}},
		{Name: "Doing", Statuses: []string{"in_progress"}, Min: 1, Max: 3},
		{Name: "Done", Statuses: []string{"done"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func RunBoards(t *testing.T, newRepo BoardFactory) {
	ctx := context.Background()
	t.Run("save, load and list by name", func(t *testing.T) {
		repo := newRepo(t)
		for _, b := range []*domain.Board{board(t, BoardA, "KYB", "Zeta"), board(t, BoardB, "KYB", "Alpha"), board(t, BoardC, "OPS", "Ops")} {
			if err := repo.SaveBoard(ctx, b); err != nil {
				t.Fatal(err)
			}
			if b.Version() != 1 {
				t.Fatalf("version after insert = %d", b.Version())
			}
		}
		got, err := repo.BoardByID(ctx, BoardA)
		if err != nil || got.Name() != "Zeta" || got.Project() != "KYB" || got.Kind() != domain.KindKanban || got.Version() != 1 ||
			!reflect.DeepEqual(got.Columns(), board(t, BoardA, "KYB", "Zeta").Columns()) {
			t.Fatalf("loaded %+v, %v", got, err)
		}
		list, err := repo.BoardsByProject(ctx, "KYB")
		if err != nil || len(list) != 2 || list[0].Name() != "Alpha" || list[1].Name() != "Zeta" {
			t.Fatalf("list = %v, %v", list, err)
		}
		if _, err := repo.BoardByID(ctx, "80000000-0000-4000-8000-0000000000ff"); !errors.Is(err, domain.ErrBoardNotFound) {
			t.Fatalf("missing err = %v", err)
		}
	})
	t.Run("update with optimistic locking", func(t *testing.T) {
		repo := newRepo(t)
		b := board(t, BoardA, "KYB", "Board")
		_ = repo.SaveBoard(ctx, b)
		stale, _ := repo.BoardByID(ctx, BoardA)
		_ = b.Configure("Renamed", domain.KindScrum, b.Columns()[:2])
		if err := repo.SaveBoard(ctx, b); err != nil || b.Version() != 2 {
			t.Fatalf("update err = %v version %d", err, b.Version())
		}
		if err := repo.SaveBoard(ctx, stale); !errors.Is(err, domain.ErrBoardConflict) {
			t.Fatalf("stale err = %v", err)
		}
		got, _ := repo.BoardByID(ctx, BoardA)
		if got.Name() != "Renamed" || got.Kind() != domain.KindScrum || len(got.Columns()) != 2 {
			t.Fatalf("updated = %+v", got)
		}
		if err := repo.SaveBoard(ctx, board(t, BoardA, "KYB", "Dup")); !errors.Is(err, domain.ErrBoardConflict) {
			t.Fatalf("duplicate insert err = %v", err)
		}
	})
	t.Run("delete", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.SaveBoard(ctx, board(t, BoardA, "KYB", "Board"))
		if err := repo.DeleteBoard(ctx, BoardA); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteBoard(ctx, BoardA); !errors.Is(err, domain.ErrBoardNotFound) {
			t.Fatalf("second delete err = %v", err)
		}
		if list, _ := repo.BoardsByProject(ctx, "KYB"); len(list) != 0 {
			t.Fatalf("list after delete = %v", list)
		}
	})
	t.Run("workspaces are isolated", func(t *testing.T) {
		repo := newRepo(t)
		_ = repo.SaveBoard(ctx, board(t, BoardA, "KYB", "Board"))
		other := tenant.With(ctx, OtherWorkspace)
		if _, err := repo.BoardByID(other, BoardA); !errors.Is(err, domain.ErrBoardNotFound) {
			t.Fatalf("leaked by id: %v", err)
		}
		if list, _ := repo.BoardsByProject(other, "KYB"); len(list) != 0 {
			t.Fatalf("leaked in list: %v", list)
		}
		if err := repo.DeleteBoard(other, BoardA); !errors.Is(err, domain.ErrBoardNotFound) {
			t.Fatalf("deleted across workspaces: %v", err)
		}
		if err := repo.SaveBoard(other, board(t, BoardA, "KYB", "Hijack")); !errors.Is(err, domain.ErrBoardConflict) {
			t.Fatalf("overwrote across workspaces: %v", err)
		}
	})
}
