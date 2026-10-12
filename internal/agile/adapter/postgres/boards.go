package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type BoardRepository struct{ pool *pgxpool.Pool }

func NewBoardRepository(pool *pgxpool.Pool) *BoardRepository { return &BoardRepository{pool: pool} }

func (r *BoardRepository) SaveBoard(ctx context.Context, b *domain.Board) error {
	cols, err := json.Marshal(b.Columns())
	if err != nil {
		return err
	}
	var tag pgconn.CommandTag
	if b.Version() == 0 {
		tag, err = r.pool.Exec(ctx, `INSERT INTO boards (id, workspace_id, project_key, name, kind, columns, version)
			VALUES ($1, $2, $3, $4, $5, $6, 1) ON CONFLICT (id) DO NOTHING`,
			string(b.ID()), tenant.Workspace(ctx), b.Project(), b.Name(), string(b.Kind()), cols)
	} else {
		tag, err = r.pool.Exec(ctx, `UPDATE boards SET name = $2, kind = $3, columns = $4, version = version + 1
			WHERE id = $1 AND version = $5 AND workspace_id = $6`,
			string(b.ID()), b.Name(), string(b.Kind()), cols, b.Version(), tenant.Workspace(ctx))
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrBoardConflict
	}
	b.MarkPersisted()
	return nil
}

const boardCols = `SELECT id::text, project_key, name, kind, columns, version FROM boards`

func scanBoard(row pgx.Row) (*domain.Board, error) {
	var id, project, name, kind string
	var raw []byte
	var version int
	if err := row.Scan(&id, &project, &name, &kind, &raw, &version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBoardNotFound
		}
		return nil, err
	}
	var cols []domain.Column
	if err := json.Unmarshal(raw, &cols); err != nil {
		return nil, err
	}
	return domain.RehydrateBoard(domain.BoardID(id), project, name, domain.BoardKind(kind), cols, version), nil
}

func (r *BoardRepository) BoardByID(ctx context.Context, id domain.BoardID) (*domain.Board, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrBoardNotFound
	}
	return scanBoard(r.pool.QueryRow(ctx, boardCols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx)))
}

func (r *BoardRepository) BoardsByProject(ctx context.Context, project string) ([]*domain.Board, error) {
	rows, err := r.pool.Query(ctx, boardCols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY name, id`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Board, error) { return scanBoard(row) })
}

func (r *BoardRepository) DeleteBoard(ctx context.Context, id domain.BoardID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrBoardNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM boards WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrBoardNotFound
	}
	return nil
}
