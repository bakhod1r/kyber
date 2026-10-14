package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Scheduled stores scheduled messages (S67).
type Scheduled struct{ pool *pgxpool.Pool }

func NewScheduled(pool *pgxpool.Pool) *Scheduled { return &Scheduled{pool: pool} }

const schedCols = `id::text, workspace_id::text, conversation_id::text, author_id::text, body, send_at, created_at`

func scanSched(row pgx.CollectableRow) (*domain.Scheduled, error) {
	var s domain.Scheduled
	if err := row.Scan(&s.ID, &s.Workspace, &s.Conversation, &s.Author, &s.Body, &s.SendAt, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.SendAt, s.CreatedAt = s.SendAt.UTC(), s.CreatedAt.UTC()
	return &s, nil
}

func (r *Scheduled) Add(ctx context.Context, s *domain.Scheduled) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO scheduled_messages (id, workspace_id, conversation_id, author_id, body, send_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, string(s.ID), string(s.Workspace), string(s.Conversation), string(s.Author), s.Body, s.SendAt, s.CreatedAt)
	return err
}

func (r *Scheduled) Update(ctx context.Context, s *domain.Scheduled) error {
	tag, err := r.pool.Exec(ctx, `UPDATE scheduled_messages SET body = $2, send_at = $3 WHERE id = $1`, string(s.ID), s.Body, s.SendAt)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrScheduledNotFound
	}
	return err
}

func (r *Scheduled) Delete(ctx context.Context, sid domain.ScheduledID) error {
	if !id.Valid(string(sid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM scheduled_messages WHERE id = $1`, string(sid))
	return err
}

func (r *Scheduled) ByID(ctx context.Context, sid domain.ScheduledID) (*domain.Scheduled, error) {
	if !id.Valid(string(sid)) {
		return nil, domain.ErrScheduledNotFound
	}
	rows, _ := r.pool.Query(ctx, `SELECT `+schedCols+` FROM scheduled_messages WHERE id = $1`, string(sid))
	s, err := pgx.CollectExactlyOneRow(rows, scanSched)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrScheduledNotFound
	}
	return s, err
}

func (r *Scheduled) Mine(ctx context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Scheduled, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+schedCols+` FROM scheduled_messages WHERE workspace_id = $1 AND author_id = $2
		ORDER BY send_at, created_at`, string(ws), string(u))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanSched)
}

func (r *Scheduled) Due(ctx context.Context, now time.Time) ([]*domain.Scheduled, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+schedCols+` FROM scheduled_messages WHERE send_at <= $1 ORDER BY send_at, created_at LIMIT 500`, now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanSched)
}

// Reminders stores personal reminders (S67).
type Reminders struct{ pool *pgxpool.Pool }

func NewReminders(pool *pgxpool.Pool) *Reminders { return &Reminders{pool: pool} }

func (r *Reminders) Add(ctx context.Context, x *domain.Reminder) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO reminders (id, workspace_id, user_id, conversation_id, text, remind_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		string(x.ID), string(x.Workspace), string(x.User), string(x.Conversation), x.Text, x.At)
	return err
}

func (r *Reminders) Due(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, workspace_id::text, user_id::text, conversation_id::text, text, remind_at
		FROM reminders WHERE reminded_at IS NULL AND remind_at <= $1 ORDER BY remind_at LIMIT 500`, now)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Reminder, error) {
		var x domain.Reminder
		if err := row.Scan(&x.ID, &x.Workspace, &x.User, &x.Conversation, &x.Text, &x.At); err != nil {
			return nil, err
		}
		x.At = x.At.UTC()
		return &x, nil
	})
}

func (r *Reminders) MarkReminded(ctx context.Context, x *domain.Reminder) error {
	events := x.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE reminders SET reminded_at = now() WHERE id = $1 AND reminded_at IS NULL`, string(x.ID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrAlreadyReminded
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}
