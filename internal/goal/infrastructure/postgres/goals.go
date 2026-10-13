// Package postgres stores goals in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/goal/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, g *domain.Goal) error {
	sql := ""
	args := []any{string(g.ID), tenant.Workspace(ctx), g.Project, g.Owner, g.Name, g.Target, g.JQL, string(g.Status)}
	if g.Version == 0 {
		sql = `INSERT INTO goals (id, workspace_id, project_key, owner_id, name, target_date, jql, status, version)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::date, $7, $8, 1) ON CONFLICT (id) DO NOTHING`
	} else {
		sql = `UPDATE goals SET name = $5, target_date = NULLIF($6, '')::date, jql = $7, status = $8, version = version + 1
			WHERE id = $1 AND workspace_id = $2 AND project_key = $3 AND owner_id = $4 AND version = $9`
		args = append(args, g.Version)
	}
	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrGoalConflict
	}
	g.Version++
	return nil
}

const cols = `SELECT id::text, project_key, owner_id::text, name, COALESCE(to_char(target_date, 'YYYY-MM-DD'), ''), jql, status, version FROM goals`

func scan(row pgx.CollectableRow) (*domain.Goal, error) {
	var g domain.Goal
	return &g, row.Scan(&g.ID, &g.Project, &g.Owner, &g.Name, &g.Target, &g.JQL, &g.Status, &g.Version)
}

func (r *Repository) ByID(ctx context.Context, id domain.GoalID) (*domain.Goal, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrGoalNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	g, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGoalNotFound
	}
	return g, err
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Goal, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND project_key = $2
		ORDER BY target_date NULLS LAST, name COLLATE "C", id`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Delete(ctx context.Context, id domain.GoalID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrGoalNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM goals WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrGoalNotFound
	}
	return nil
}
