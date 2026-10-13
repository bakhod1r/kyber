// Package postgres is the PostgreSQL activity log.
package postgres

import (
	"context"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/insights/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Record(ctx context.Context, entries ...domain.Entry) error {
	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`INSERT INTO activity (event, at, project_key, issue_id, kind, from_val, to_val, from_points, to_points, workspace_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
			e.Event, e.At, e.Project, e.Issue, string(e.Kind), e.From, e.To, e.FromPoints, e.ToPoints, tenant.Workspace(ctx))
	}
	return r.pool.SendBatch(ctx, batch).Close()
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]domain.Entry, error) {
	rows, err := r.pool.Query(ctx, `SELECT event, at, project_key, issue_id, kind, from_val, to_val, from_points, to_points
		FROM activity WHERE project_key = $1 AND workspace_id = $2 ORDER BY at, event`, project, tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Entry, error) {
		var e domain.Entry
		err := row.Scan(&e.Event, &e.At, &e.Project, &e.Issue, &e.Kind, &e.From, &e.To, &e.FromPoints, &e.ToPoints)
		return e, err
	})
}
