// Package postgres is the PostgreSQL permission-scheme store.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ProjectGrants(ctx context.Context, project string) (domain.GrantTable, bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT permission, holder_kind, holder_value FROM project_permission_grants
		WHERE workspace_id = $1 AND project_key = $2 ORDER BY permission, holder_kind, holder_value`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	t, found := domain.GrantTable{}, false
	for rows.Next() {
		var p, kind, value string
		if err := rows.Scan(&p, &kind, &value); err != nil {
			return nil, false, err
		}
		g, err := domain.ParseGrant(kind, value)
		if err != nil {
			return nil, false, err
		}
		t[domain.Permission(p)] = append(t[domain.Permission(p)], g)
		found = true
	}
	return t, found, rows.Err()
}

// SaveProjectGrants replaces the project's grants atomically.
func (r *Repository) SaveProjectGrants(ctx context.Context, project string, t domain.GrantTable) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ws := tenant.Workspace(ctx)
		if _, err := tx.Exec(ctx, `DELETE FROM project_permission_grants WHERE workspace_id = $1 AND project_key = $2`, ws, project); err != nil {
			return err
		}
		for p, gs := range t {
			for _, g := range gs {
				if _, err := tx.Exec(ctx, `INSERT INTO project_permission_grants (workspace_id, project_key, permission, holder_kind, holder_value)
					VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, ws, project, string(p), g.Kind(), g.Value()); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repository) DeleteProjectGrants(ctx context.Context, project string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM project_permission_grants WHERE workspace_id = $1 AND project_key = $2`, tenant.Workspace(ctx), project)
	return err
}
