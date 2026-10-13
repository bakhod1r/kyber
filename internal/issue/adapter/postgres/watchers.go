package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

type WatcherRepository struct{ pool *pgxpool.Pool }

func NewWatcherRepository(pool *pgxpool.Pool) *WatcherRepository {
	return &WatcherRepository{pool: pool}
}

func (r *WatcherRepository) Watch(ctx context.Context, issue domain.IssueID, user domain.UserID) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO issue_watchers (issue_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		string(issue), string(user))
	return err
}

func (r *WatcherRepository) Unwatch(ctx context.Context, issue domain.IssueID, user domain.UserID) error {
	if !kid.Valid(string(user)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM issue_watchers WHERE issue_id = $1 AND user_id = $2`, string(issue), string(user))
	return err
}

func (r *WatcherRepository) Watchers(ctx context.Context, issue domain.IssueID) ([]domain.UserID, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id::text FROM issue_watchers WHERE issue_id = $1 ORDER BY watched_at, user_id`, string(issue))
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	out := make([]domain.UserID, len(ids))
	for i, id := range ids {
		out[i] = domain.UserID(id)
	}
	return out, err
}
