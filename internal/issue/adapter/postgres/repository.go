// Package postgres is the PostgreSQL Issue repository with a transactional outbox.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, is *domain.Issue, events []domain.Event) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var tag interface{ RowsAffected() int64 }
		var err error
		if is.Version() == 0 {
			tag, err = tx.Exec(ctx, `INSERT INTO issues (id, project_key, number, title, type, status,
				description, priority, assignee_id, rank, sprint_id, reporter_id, estimate_tenths, version)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1) ON CONFLICT DO NOTHING`,
				string(is.ID()), is.Key().Project(), is.Key().Number(), is.Title(), string(is.Type()), string(is.Status()),
				is.Description(), string(is.Priority()), nullable(string(is.Assignee())), string(is.Rank()),
				nullable(string(is.Sprint())), nullable(string(is.Reporter())), estimateCol(is))
		} else {
			tag, err = tx.Exec(ctx, `UPDATE issues SET title = $2, type = $3, status = $4, description = $5,
				priority = $6, assignee_id = $7, rank = $8, sprint_id = $9, estimate_tenths = $11,
				version = version + 1, updated_at = now()
				WHERE id = $1 AND version = $10`,
				string(is.ID()), is.Title(), string(is.Type()), string(is.Status()), is.Description(),
				string(is.Priority()), nullable(string(is.Assignee())), string(is.Rank()), nullable(string(is.Sprint())),
				is.Version(), estimateCol(is))
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrConcurrentModification
		}
		return db.WriteOutbox(ctx, tx, events)
	})
	if err != nil {
		return err
	}
	is.MarkPersisted()
	return nil
}

const selectCols = `SELECT id::text, project_key, number, title, type, status, description, priority,
	COALESCE(assignee_id::text, ''), rank, COALESCE(sprint_id::text, ''), COALESCE(reporter_id::text, ''), estimate_tenths, version FROM issues`

func (r *Repository) ByKey(ctx context.Context, key domain.IssueKey) (*domain.Issue, error) {
	is, err := scan(r.pool.QueryRow(ctx, selectCols+` WHERE project_key = $1 AND number = $2`, key.Project(), key.Number()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrIssueNotFound
	}
	return is, err
}

func (r *Repository) ListByProject(ctx context.Context, project string, f domain.ListFilter) ([]*domain.Issue, error) {
	where, args := `WHERE project_key = $1`, []any{project}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		where += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	if f.Sprint != nil {
		if *f.Sprint == "" {
			where += ` AND sprint_id IS NULL`
		} else {
			args = append(args, string(*f.Sprint))
			where += fmt.Sprintf(` AND sprint_id = $%d`, len(args))
		}
	}
	rows, err := r.pool.Query(ctx, selectCols+" "+where+` ORDER BY rank, number`, args...)
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

func (r *Repository) LastRank(ctx context.Context, project string) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE project_key = $1 ORDER BY rank DESC LIMIT 1`, project)
}

func (r *Repository) NextRank(ctx context.Context, project string, after domain.Rank) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE project_key = $1 AND rank > $2 ORDER BY rank LIMIT 1`, project, string(after))
}

func (r *Repository) PrevRank(ctx context.Context, project string, before domain.Rank) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE project_key = $1 AND rank < $2 ORDER BY rank DESC LIMIT 1`, project, string(before))
}

func (r *Repository) rankQuery(ctx context.Context, sql string, args ...any) (domain.Rank, error) {
	var rank string
	err := r.pool.QueryRow(ctx, sql, args...).Scan(&rank)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return domain.Rank(rank), err
}

func scan(row pgx.Row) (*domain.Issue, error) {
	var s domain.Snapshot
	var project string
	var number int
	if err := row.Scan(&s.ID, &project, &number, &s.Title, &s.Type, &s.Status, &s.Description,
		&s.Priority, &s.Assignee, &s.Rank, &s.Sprint, &s.Reporter, &s.Estimate, &s.Version); err != nil {
		return nil, err
	}
	key, err := domain.NewIssueKey(project, number)
	if err != nil {
		return nil, err
	}
	s.Key = key
	return domain.Rehydrate(s), nil
}

func estimateCol(is *domain.Issue) *int {
	p, ok := is.Estimate()
	if !ok {
		return nil
	}
	v := int(p)
	return &v
}

// nullable maps the domain's "" (unassigned / backlog) to SQL NULL.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
