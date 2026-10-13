// Package postgres stores saved filters in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/search/domain"
)

type FilterRepository struct{ pool *pgxpool.Pool }

func NewFilterRepository(pool *pgxpool.Pool) *FilterRepository { return &FilterRepository{pool: pool} }

func (r *FilterRepository) SaveFilter(ctx context.Context, f *domain.Filter) error {
	var n int64
	if f.Version == 0 {
		tag, err := r.pool.Exec(ctx, `INSERT INTO saved_filters (id, workspace_id, owner_id, name, jql, shared, version)
			VALUES ($1, $2, $3, $4, $5, $6, 1) ON CONFLICT (id) DO NOTHING`,
			string(f.ID), tenant.Workspace(ctx), f.Owner, f.Name, f.JQL, f.Shared)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
	} else {
		tag, err := r.pool.Exec(ctx, `UPDATE saved_filters SET name = $2, jql = $3, shared = $4, version = version + 1
			WHERE id = $1 AND version = $5 AND workspace_id = $6`, string(f.ID), f.Name, f.JQL, f.Shared, f.Version, tenant.Workspace(ctx))
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
	}
	if n != 1 {
		return domain.ErrFilterConflict
	}
	f.Version++
	return nil
}

const cols = `SELECT id::text, owner_id::text, name, jql, shared, version FROM saved_filters`

func scan(row pgx.CollectableRow) (*domain.Filter, error) {
	var f domain.Filter
	return &f, row.Scan(&f.ID, &f.Owner, &f.Name, &f.JQL, &f.Shared, &f.Version)
}

func (r *FilterRepository) FilterByID(ctx context.Context, id domain.FilterID) (*domain.Filter, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrFilterNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrFilterNotFound
	}
	return f, err
}

func (r *FilterRepository) FiltersFor(ctx context.Context, user string) ([]*domain.Filter, error) {
	if !kid.Valid(user) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND (owner_id = $2 OR shared) ORDER BY name COLLATE "C", id`,
		tenant.Workspace(ctx), user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *FilterRepository) DeleteFilter(ctx context.Context, id domain.FilterID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrFilterNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM saved_filters WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFilterNotFound
	}
	return nil
}
