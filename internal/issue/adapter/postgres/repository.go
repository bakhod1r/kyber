// Package postgres is the PostgreSQL Issue repository with a transactional outbox.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, is *domain.Issue, events []domain.Event) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var tag interface{ RowsAffected() int64 }
		var err error
		if is.Version() == 0 {
			tag, err = tx.Exec(ctx, `INSERT INTO issues (id, project_key, number, title, type, status, version)
				VALUES ($1, $2, $3, $4, $5, $6, 1) ON CONFLICT DO NOTHING`,
				string(is.ID()), is.Key().Project(), is.Key().Number(), is.Title(), string(is.Type()), string(is.Status()))
		} else {
			tag, err = tx.Exec(ctx, `UPDATE issues SET title = $2, type = $3, status = $4,
				version = version + 1, updated_at = now() WHERE id = $1 AND version = $5`,
				string(is.ID()), is.Title(), string(is.Type()), string(is.Status()), is.Version())
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrConcurrentModification
		}
		for _, e := range events {
			payload, err := json.Marshal(e)
			if err != nil {
				return fmt.Errorf("marshal %s: %w", e.EventName(), err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO outbox (name, payload) VALUES ($1, $2)`, e.EventName(), payload); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	is.MarkPersisted()
	return nil
}

const selectCols = `SELECT id::text, project_key, number, title, type, status, version FROM issues`

func (r *Repository) ByKey(ctx context.Context, key domain.IssueKey) (*domain.Issue, error) {
	is, err := scan(r.pool.QueryRow(ctx, selectCols+` WHERE project_key = $1 AND number = $2`, key.Project(), key.Number()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrIssueNotFound
	}
	return is, err
}

func (r *Repository) ListByProject(ctx context.Context, project string, status *domain.StatusID) ([]*domain.Issue, error) {
	rows, err := r.pool.Query(ctx, selectCols+` WHERE project_key = $1 AND ($2::text IS NULL OR status = $2)
		ORDER BY number`, project, (*string)(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Issue
	for rows.Next() {
		is, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}

func scan(row pgx.Row) (*domain.Issue, error) {
	var id, project, title, typ, status string
	var number, version int
	if err := row.Scan(&id, &project, &number, &title, &typ, &status, &version); err != nil {
		return nil, err
	}
	key, err := domain.NewIssueKey(project, number)
	if err != nil {
		return nil, err
	}
	return domain.Rehydrate(domain.IssueID(id), key, title, domain.IssueType(typ), domain.StatusID(status), version), nil
}
