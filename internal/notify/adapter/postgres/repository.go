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
	_, err := r.pool.Exec(ctx, `INSERT INTO notifications (id, recipient_id, workspace_id, kind, issue_key, issue_title, actor_name,
		excerpt, source_event, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (source_event, recipient_id) DO NOTHING`,
		string(n.ID), string(n.Recipient), n.Workspace, string(n.Kind), n.IssueKey, n.IssueTitle, n.ActorName, n.Excerpt, n.SourceEvent, n.CreatedAt)
	return err
}

// valid guards the uuid casts: an inbox with a malformed id is simply empty.
func valid(in domain.Inbox) bool { return id.Valid(string(in.User)) && id.Valid(in.Workspace) }

func (r *Repository) ListFor(ctx context.Context, in domain.Inbox, unreadOnly bool, limit int) ([]domain.Notification, error) {
	if !valid(in) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text, recipient_id::text, workspace_id::text, kind, issue_key, issue_title, actor_name,
		excerpt, source_event, created_at, read_at FROM notifications
		WHERE recipient_id = $1 AND workspace_id = $4 AND (NOT $2 OR read_at IS NULL) ORDER BY created_at DESC, id LIMIT $3`,
		string(in.User), unreadOnly, limit, in.Workspace)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Notification, error) {
		var n domain.Notification
		var readAt *time.Time
		err := row.Scan(&n.ID, &n.Recipient, &n.Workspace, &n.Kind, &n.IssueKey, &n.IssueTitle, &n.ActorName, &n.Excerpt,
			&n.SourceEvent, &n.CreatedAt, &readAt)
		if readAt != nil {
			n.ReadAt = *readAt
		}
		return n, err
	})
}

func (r *Repository) UnreadCount(ctx context.Context, in domain.Inbox) (int, error) {
	if !valid(in) {
		return 0, nil
	}
	var c int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND workspace_id = $2 AND read_at IS NULL`,
		string(in.User), in.Workspace).Scan(&c)
	return c, err
}

func (r *Repository) MarkRead(ctx context.Context, in domain.Inbox, nid domain.NotificationID, at time.Time) error {
	if !id.Valid(string(nid)) || !valid(in) {
		return domain.ErrNotificationNotFound
	}
	tag, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = COALESCE(read_at, $3)
		WHERE id = $1 AND recipient_id = $2 AND workspace_id = $4`, string(nid), string(in.User), at, in.Workspace)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotificationNotFound
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, in domain.Inbox, at time.Time) error {
	if !valid(in) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE notifications SET read_at = $2 WHERE recipient_id = $1 AND workspace_id = $3 AND read_at IS NULL`,
		string(in.User), at, in.Workspace)
	return err
}
