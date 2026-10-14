package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Bots stores the bot list (S84).
type Bots struct{ pool *pgxpool.Pool }

func NewBots(pool *pgxpool.Pool) *Bots { return &Bots{pool: pool} }

func (r *Bots) Add(ctx context.Context, b *domain.Bot) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO chat_bots (workspace_id, user_id, name, created_by, created_at) VALUES ($1, $2, $3, $4, $5)`,
		string(b.Workspace), string(b.User), b.Name, string(b.CreatedBy), b.CreatedAt)
	return err
}

func (r *Bots) In(ctx context.Context, ws domain.WorkspaceID) ([]*domain.Bot, error) {
	if !id.Valid(string(ws)) {
		return []*domain.Bot{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT workspace_id::text, user_id::text, name, created_by::text, created_at FROM chat_bots
		WHERE workspace_id = $1 ORDER BY created_at, user_id`, string(ws))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Bot, error) {
		var b domain.Bot
		if err := row.Scan(&b.Workspace, &b.User, &b.Name, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.CreatedAt = b.CreatedAt.UTC()
		return &b, nil
	})
}
