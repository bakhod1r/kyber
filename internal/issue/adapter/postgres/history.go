package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type HistoryRepository struct{ pool *pgxpool.Pool }

func NewHistoryRepository(pool *pgxpool.Pool) *HistoryRepository {
	return &HistoryRepository{pool: pool}
}

func (r *HistoryRepository) Record(ctx context.Context, entries ...domain.HistoryEntry) error {
	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`INSERT INTO issue_history (workspace_id, event, seq, issue_id, at, by_user, field, from_val, to_val)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`,
			tenant.Workspace(ctx), e.Event, e.Seq, string(e.Issue), e.At, string(e.By), e.Field, e.From, e.To)
	}
	return r.pool.SendBatch(ctx, batch).Close()
}

func (r *HistoryRepository) ForIssue(ctx context.Context, issue domain.IssueID) ([]domain.HistoryEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT event, seq, issue_id, at, by_user, field, from_val, to_val FROM issue_history
		WHERE workspace_id = $1 AND issue_id = $2 ORDER BY at, event, seq`, tenant.Workspace(ctx), string(issue))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.HistoryEntry, error) {
		var e domain.HistoryEntry
		var id, by string
		err := row.Scan(&e.Event, &e.Seq, &id, &e.At, &by, &e.Field, &e.From, &e.To)
		e.Issue, e.By = domain.IssueID(id), domain.UserID(by)
		return e, err
	})
}
