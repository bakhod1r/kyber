package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

var ErrLastBoard = errors.New("a project keeps at least one board")

// Admins checks the project administration permission (ACL to the Project context).
type Admins interface {
	Administer(ctx context.Context, actor, project string) error
}

// WorkflowStatus is a status of the project's workflow, as the Agile context sees it.
type WorkflowStatus struct{ ID, Name string }

// Workflows is the anti-corruption port to Issue Tracking's workflows.
type Workflows interface {
	Statuses(ctx context.Context, project string) ([]WorkflowStatus, error)
}

// BoardSpec is a board's configuration; no columns means one column per workflow status.
type BoardSpec struct {
	Name    string
	Kind    domain.BoardKind
	Columns []domain.Column
}

// Boards lists the project's boards; like Jira, a project gets a Kanban board on first use.
func (s *Service) Boards(ctx context.Context, actor, project string) ([]*domain.Board, error) {
	if err := s.d.Access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	list, err := s.d.Boards.BoardsByProject(ctx, project)
	if err != nil || len(list) > 0 {
		return list, err
	}
	b, err := s.newBoard(ctx, project, BoardSpec{Name: project + " board", Kind: domain.KindKanban})
	if err != nil {
		return nil, err
	}
	return []*domain.Board{b}, s.d.Boards.SaveBoard(ctx, b)
}

func (s *Service) CreateBoard(ctx context.Context, actor, project string, spec BoardSpec) (*domain.Board, error) {
	if err := s.d.Admins.Administer(ctx, actor, project); err != nil {
		return nil, err
	}
	b, err := s.newBoard(ctx, project, spec)
	if err != nil {
		return nil, err
	}
	return b, s.d.Boards.SaveBoard(ctx, b)
}

// Board returns a board the actor may see; non-members get ErrBoardNotFound.
func (s *Service) Board(ctx context.Context, actor, id string) (*domain.Board, error) {
	return s.loadBoard(ctx, id, func(project string) error { return s.d.Access.Authorize(ctx, actor, project, false) })
}

func (s *Service) UpdateBoard(ctx context.Context, actor, id string, version int, spec BoardSpec) (*domain.Board, error) {
	b, err := s.loadBoard(ctx, id, func(project string) error { return s.d.Admins.Administer(ctx, actor, project) })
	if err != nil {
		return nil, err
	}
	if b.Version() != version {
		return nil, domain.ErrBoardConflict
	}
	cols, err := s.columns(ctx, b.Project(), spec.Columns)
	if err != nil {
		return nil, err
	}
	if err := b.Configure(spec.Name, spec.Kind, cols); err != nil {
		return nil, err
	}
	return b, s.d.Boards.SaveBoard(ctx, b)
}

func (s *Service) DeleteBoard(ctx context.Context, actor, id string) error {
	b, err := s.loadBoard(ctx, id, func(project string) error { return s.d.Admins.Administer(ctx, actor, project) })
	if err != nil {
		return err
	}
	list, err := s.d.Boards.BoardsByProject(ctx, b.Project())
	if err != nil {
		return err
	}
	if len(list) <= 1 {
		return ErrLastBoard
	}
	return s.d.Boards.DeleteBoard(ctx, b.ID())
}

func (s *Service) loadBoard(ctx context.Context, id string, allow func(project string) error) (*domain.Board, error) {
	b, err := s.d.Boards.BoardByID(ctx, domain.BoardID(id))
	if err != nil {
		return nil, err
	}
	if err := allow(b.Project()); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrBoardNotFound
		}
		return nil, err
	}
	return b, nil
}

func (s *Service) newBoard(ctx context.Context, project string, spec BoardSpec) (*domain.Board, error) {
	cols, err := s.columns(ctx, project, spec.Columns)
	if err != nil {
		return nil, err
	}
	return domain.NewBoard(domain.BoardID(s.d.NewID()), project, spec.Name, spec.Kind, cols)
}

// columns checks mapped statuses against the project's workflow, or derives one column per status.
func (s *Service) columns(ctx context.Context, project string, cols []domain.Column) ([]domain.Column, error) {
	statuses, err := s.d.Workflows.Statuses(ctx, project)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		for _, st := range statuses {
			cols = append(cols, domain.Column{Name: st.Name, Statuses: []string{st.ID}})
		}
		return cols, nil
	}
	known := map[string]bool{}
	for _, st := range statuses {
		known[st.ID] = true
	}
	for _, c := range cols {
		for _, id := range c.Statuses {
			if !known[id] {
				return nil, fmt.Errorf("status %q is not in the project's workflow: %w", id, domain.ErrInvalidBoard)
			}
		}
	}
	return cols, nil
}
