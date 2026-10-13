// Package postgres is the PostgreSQL import mapping store.
package postgres

import (
	"context"
	"errors"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Add(ctx context.Context, m domain.Mapping) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO import_mappings (project_key, external_key, issue_key, workspace_id) VALUES ($1, $2, $3, $4)`,
		m.Project, m.ExternalKey, m.IssueKey, tenant.Workspace(ctx))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrAlreadyImported
	}
	return err
}

func (r *Repository) ForProject(ctx context.Context, project string) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT external_key, issue_key FROM import_mappings WHERE project_key = $1 AND workspace_id = $2`, project, tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var ext, key string
		if err := rows.Scan(&ext, &key); err != nil {
			return nil, err
		}
		out[ext] = key
	}
	return out, rows.Err()
}
