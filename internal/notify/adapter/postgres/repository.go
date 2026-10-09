// Package postgres is the PostgreSQL notification store.
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Add(ctx context.Context, n domain.Notification) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notifications (id, recipient_id, kind, issue_key, issue_title, actor_name,
		excerpt, source_event, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (source_event, recipient_id) DO NOTHING`,
		string(n.ID), string(n.Recipient), string(n.Kind), n.IssueKey, n.IssueTitle, n.ActorName, n.Excerpt, n.SourceEvent, n.CreatedAt)
	return err
}

func (r *Repository) ListFor(ctx context.Context, user domain.UserID, unreadOnly bool, limit int) ([]domain.Notification, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, recipient_id::text, kind, issue_key, issue_title, actor_name,
		excerpt, source_event, created_at, read_at FROM notifications
		WHERE recipient_id = $1 AND (NOT $2 OR read_at IS NULL) ORDER BY created_at DESC, id LIMIT $3`,
		string(user), unreadOnly, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Notification, error) {
		var n domain.Notification
		var readAt *time.Time
		err := row.Scan(&n.ID, &n.Recipient, &n.Kind, &n.IssueKey, &n.IssueTitle, &n.ActorName, &n.Excerpt,
			&n.SourceEvent, &n.CreatedAt, &readAt)
		if readAt != nil {
			n.ReadAt = *readAt
		}
		return n, err
	})
}

func (r *Repository) UnreadCount(ctx context.Context, user domain.UserID) (int, error) {
	var c int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND read_at IS NULL`, string(user)).Scan(&c)
	return c, err
}

func (r *Repository) MarkRead(ctx context.Context, user domain.UserID, nid domain.NotificationID, at time.Time) error {
	if !id.Valid(string(nid)) {
		return domain.ErrNotificationNotFound
	}
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = COALESCE(read_at, $3)
		WHERE id = $1 AND recipient_id = $2`, string(nid), string(user), at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotificationNotFound
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, user domain.UserID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = $2 WHERE recipient_id = $1 AND read_at IS NULL`, string(user), at)
	return err
}
