// Package postgres stores pages and their revisions in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, p *domain.Page, rev domain.Revision) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var (
			n   int64
			err error
		)
		if p.Version == 0 {
			tag, e := tx.Exec(ctx, `INSERT INTO doc_pages (id, workspace_id, project_key, title, body, author_id, updated_by, updated_at, version)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO NOTHING`,
				string(p.ID), tenant.Workspace(ctx), p.Project, p.Title, p.Body, p.Author, p.UpdatedBy, p.UpdatedAt, rev.Number)
			n, err = tag.RowsAffected(), e
		} else {
			tag, e := tx.Exec(ctx, `UPDATE doc_pages SET title = $3, body = $4, updated_by = $5, updated_at = $6, version = $7
				WHERE id = $1 AND workspace_id = $2 AND version = $8`,
				string(p.ID), tenant.Workspace(ctx), p.Title, p.Body, p.UpdatedBy, p.UpdatedAt, rev.Number, p.Version)
			n, err = tag.RowsAffected(), e
		}
		if err != nil {
			return err
		}
		if n != 1 {
			return domain.ErrPageConflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO doc_revisions (page_id, number, title, body, by_id, at) VALUES ($1, $2, $3, $4, $5, $6)`,
			string(p.ID), rev.Number, rev.Title, rev.Body, rev.By, rev.At)
		return err
	})
	if err != nil {
		return err
	}
	p.Version = rev.Number
	return nil
}

const cols = `SELECT id::text, project_key, title, body, author_id::text, updated_by::text, updated_at, version FROM doc_pages`

func scan(row pgx.CollectableRow) (*domain.Page, error) {
	var p domain.Page
	err := row.Scan(&p.ID, &p.Project, &p.Title, &p.Body, &p.Author, &p.UpdatedBy, &p.UpdatedAt, &p.Version)
	p.UpdatedAt = p.UpdatedAt.UTC()
	return &p, err
}

func (r *Repository) ByID(ctx context.Context, id domain.PageID) (*domain.Page, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrPageNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPageNotFound
	}
	return p, err
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Page, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY title COLLATE "C", id`,
		tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) History(ctx context.Context, id domain.PageID) ([]domain.Revision, error) {
	if !kid.Valid(string(id)) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT v.page_id::text, v.number, v.title, v.body, v.by_id::text, v.at
		FROM doc_revisions v JOIN doc_pages p ON p.id = v.page_id
		WHERE v.page_id = $1 AND p.workspace_id = $2 ORDER BY v.number DESC`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Revision, error) {
		var v domain.Revision
		err := row.Scan(&v.Page, &v.Number, &v.Title, &v.Body, &v.By, &v.At)
		v.At = v.At.UTC()
		return v, err
	})
}

func (r *Repository) Delete(ctx context.Context, id domain.PageID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrPageNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM doc_pages WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPageNotFound
	}
	return nil
}
