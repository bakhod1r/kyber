package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

type WorklogRepository struct{ pool *pgxpool.Pool }

func NewWorklogRepository(pool *pgxpool.Pool) *WorklogRepository {
	return &WorklogRepository{pool: pool}
}

func (r *WorklogRepository) Add(ctx context.Context, w domain.Worklog) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO issue_worklogs (id, issue_id, author_id, started, seconds, comment)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
		string(w.ID), string(w.Issue), nullable(string(w.Author)), w.Started, int64(w.Seconds), w.Comment)
	return err
}

func (r *WorklogRepository) Remove(ctx context.Context, id domain.WorklogID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM issue_worklogs WHERE id = $1`, string(id))
	return err
}

const worklogCols = `SELECT w.id::text, w.issue_id::text, i.project_key, i.number, COALESCE(w.author_id::text, ''), w.started, w.seconds, w.comment
	FROM issue_worklogs w JOIN issues i ON i.id = w.issue_id`

func scanWorklog(row pgx.CollectableRow) (domain.Worklog, error) {
	var w domain.Worklog
	var project string
	var number int
	if err := row.Scan(&w.ID, &w.Issue, &project, &number, &w.Author, &w.Started, &w.Seconds, &w.Comment); err != nil {
		return domain.Worklog{}, err
	}
	w.Key, _ = domain.NewIssueKey(project, number)
	w.Started = w.Started.UTC()
	return w, nil
}

func (r *WorklogRepository) ByID(ctx context.Context, id domain.WorklogID) (domain.Worklog, error) {
	if !kid.Valid(string(id)) {
		return domain.Worklog{}, domain.ErrWorklogNotFound
	}
	rows, err := r.pool.Query(ctx, worklogCols+` WHERE w.id = $1`, string(id))
	if err != nil {
		return domain.Worklog{}, err
	}
	w, err := pgx.CollectExactlyOneRow(rows, scanWorklog)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worklog{}, domain.ErrWorklogNotFound
	}
	return w, err
}

func (r *WorklogRepository) ForIssue(ctx context.Context, issue domain.IssueID) ([]domain.Worklog, error) {
	rows, err := r.pool.Query(ctx, worklogCols+` WHERE w.issue_id = $1 ORDER BY w.started, w.id`, string(issue))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanWorklog)
}
