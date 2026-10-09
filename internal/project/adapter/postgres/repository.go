// Package postgres is the PostgreSQL Project repository.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const uniqueViolation = "23505"

func (r *Repository) Create(ctx context.Context, p *domain.Project) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO projects (id, key, name, issue_seq) VALUES ($1, $2, $3, $4)`,
		string(p.ID()), p.Key(), p.Name(), p.IssueSeq())
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.ErrKeyTaken
	}
	return err
}

// Update locks the project row (SELECT … FOR UPDATE) for the duration of fn.
func (r *Repository) Update(ctx context.Context, key string, fn func(*domain.Project) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		p, err := scan(tx.QueryRow(ctx, selectCols+` WHERE key = $1 FOR UPDATE`, key))
		if err != nil {
			return err
		}
		if err := fn(p); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE projects SET name = $2, issue_seq = $3 WHERE key = $1`, p.Key(), p.Name(), p.IssueSeq())
		return err
	})
}

func (r *Repository) ByKey(ctx context.Context, key string) (*domain.Project, error) {
	return scan(r.pool.QueryRow(ctx, selectCols+` WHERE key = $1`, key))
}

func (r *Repository) List(ctx context.Context) ([]*domain.Project, error) {
	rows, err := r.pool.Query(ctx, selectCols+` ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Project
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const selectCols = `SELECT id::text, key, name, issue_seq FROM projects`

func scan(row pgx.Row) (*domain.Project, error) {
	var id, key, name string
	var seq int
	if err := row.Scan(&id, &key, &name, &seq); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, err
	}
	return domain.Rehydrate(domain.ProjectID(id), key, name, seq), nil
}
