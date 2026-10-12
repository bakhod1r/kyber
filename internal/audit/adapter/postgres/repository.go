// Package postgres is the pgx audit log.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Append(ctx context.Context, e domain.Entry) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO audit_log (workspace_id, at, actor_id, action, target, project_key, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, tenant.Workspace(ctx), e.At, e.Actor, e.Action, e.Target, e.Project, e.Status)
	return err
}

func (r *Repository) ForProject(ctx context.Context, project string, limit int) ([]domain.Entry, error) {
	rows, err := r.pool.Query(ctx, `SELECT at, actor_id, action, target, project_key, status FROM audit_log
		WHERE workspace_id = $1 AND project_key = $2 ORDER BY at DESC, id DESC LIMIT $3`, tenant.Workspace(ctx), project, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Entry, error) {
		var e domain.Entry
		err := row.Scan(&e.At, &e.Actor, &e.Action, &e.Target, &e.Project, &e.Status)
		e.At = e.At.UTC()
		return e, err
	})
}
