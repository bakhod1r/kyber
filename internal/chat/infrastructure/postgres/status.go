package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Statuses stores statuses (S66).
type Statuses struct{ pool *pgxpool.Pool }

func NewStatuses(pool *pgxpool.Pool) *Statuses { return &Statuses{pool: pool} }

func orNull(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (r *Statuses) Put(ctx context.Context, s *domain.Status) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO user_statuses (workspace_id, user_id, emoji, text, expires_at, dnd_until)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET emoji = $3, text = $4, expires_at = $5, dnd_until = $6`,
		string(s.Workspace), string(s.User), s.Emoji, s.Text, orNull(s.ExpiresAt), orNull(s.DNDUntil))
	return err
}

func (r *Statuses) Of(ctx context.Context, ws domain.WorkspaceID, users []domain.UserID) ([]*domain.Status, error) {
	ids := []string{}
	for _, u := range users {
		if id.Valid(string(u)) {
			ids = append(ids, string(u))
		}
	}
	if !id.Valid(string(ws)) || len(ids) == 0 {
		return []*domain.Status{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT workspace_id::text, user_id::text, emoji, text, expires_at, dnd_until FROM user_statuses
		WHERE workspace_id = $1 AND user_id = ANY($2::uuid[]) ORDER BY user_id`, string(ws), ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Status, error) {
		var s domain.Status
		var exp, dnd *time.Time
		if err := row.Scan(&s.Workspace, &s.User, &s.Emoji, &s.Text, &exp, &dnd); err != nil {
			return nil, err
		}
		if exp != nil {
			s.ExpiresAt = exp.UTC()
		}
		if dnd != nil {
			s.DNDUntil = dnd.UTC()
		}
		return &s, nil
	})
}
