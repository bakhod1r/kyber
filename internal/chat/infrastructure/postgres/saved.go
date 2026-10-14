package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Saved stores saved messages (S65).
type Saved struct{ pool *pgxpool.Pool }

func NewSaved(pool *pgxpool.Pool) *Saved { return &Saved{pool: pool} }

func remindAt(s *domain.Saved) *time.Time {
	if s.RemindAt.IsZero() {
		return nil
	}
	return &s.RemindAt
}

func (r *Saved) Put(ctx context.Context, s *domain.Saved) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO saved_messages (workspace_id, user_id, message_id, conversation_id, saved_at, remind_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workspace_id, user_id, message_id) DO UPDATE SET saved_at = $5, remind_at = $6, reminded_at = NULL`,
		string(s.Workspace), string(s.User), string(s.Message), string(s.Conversation), s.SavedAt, remindAt(s))
	return err
}

func (r *Saved) Remove(ctx context.Context, ws domain.WorkspaceID, u domain.UserID, m domain.MessageID) error {
	if !id.Valid(string(m)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM saved_messages WHERE workspace_id = $1 AND user_id = $2 AND message_id = $3`, string(ws), string(u), string(m))
	return err
}

const savedCols = `s.workspace_id::text, s.user_id::text, s.message_id::text, s.conversation_id::text, s.saved_at, s.remind_at, s.reminded_at IS NOT NULL, m.body`

func scanSaved(row pgx.CollectableRow) (*domain.Saved, error) {
	var s domain.Saved
	var at *time.Time
	var body string
	if err := row.Scan(&s.Workspace, &s.User, &s.Message, &s.Conversation, &s.SavedAt, &at, &s.Reminded, &body); err != nil {
		return nil, err
	}
	s.SavedAt = s.SavedAt.UTC()
	if at != nil {
		s.RemindAt = at.UTC()
	}
	return s.WithExcerpt(body), nil
}

func (r *Saved) For(ctx context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Saved, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+savedCols+` FROM saved_messages s JOIN messages m ON m.id = s.message_id
		WHERE s.workspace_id = $1 AND s.user_id = $2 ORDER BY s.saved_at DESC`, string(ws), string(u))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanSaved)
}

func (r *Saved) Due(ctx context.Context, now time.Time) ([]*domain.Saved, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+savedCols+` FROM saved_messages s JOIN messages m ON m.id = s.message_id
		WHERE s.remind_at IS NOT NULL AND s.reminded_at IS NULL AND s.remind_at <= $1 ORDER BY s.remind_at LIMIT 500`, now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanSaved)
}

func (r *Saved) MarkReminded(ctx context.Context, s *domain.Saved) error {
	events := s.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE saved_messages SET reminded_at = now()
			WHERE workspace_id = $1 AND user_id = $2 AND message_id = $3 AND reminded_at IS NULL`,
			string(s.Workspace), string(s.User), string(s.Message))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrAlreadyReminded
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}
