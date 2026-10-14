package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Webhooks stores incoming webhooks (S81).
type Webhooks struct{ pool *pgxpool.Pool }

func NewWebhooks(pool *pgxpool.Pool) *Webhooks { return &Webhooks{pool: pool} }

const webhookCols = `id::text, workspace_id::text, conversation_id::text, name, secret_hash, created_by::text, created_at`

func scanWebhook(row pgx.CollectableRow) (*domain.Webhook, error) {
	var h domain.Webhook
	if err := row.Scan(&h.ID, &h.Workspace, &h.Conversation, &h.Name, &h.SecretHash, &h.CreatedBy, &h.CreatedAt); err != nil {
		return nil, err
	}
	h.CreatedAt = h.CreatedAt.UTC()
	return &h, nil
}

func (r *Webhooks) Add(ctx context.Context, h *domain.Webhook) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO chat_webhooks (id, workspace_id, conversation_id, name, secret_hash, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, string(h.ID), string(h.Workspace), string(h.Conversation), h.Name, h.SecretHash, string(h.CreatedBy), h.CreatedAt)
	return err
}

func (r *Webhooks) Update(ctx context.Context, h *domain.Webhook) error {
	tag, err := r.pool.Exec(ctx, `UPDATE chat_webhooks SET secret_hash = $2 WHERE id = $1`, string(h.ID), h.SecretHash)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrWebhookNotFound
	}
	return err
}

func (r *Webhooks) Remove(ctx context.Context, hid domain.WebhookID) error {
	if !id.Valid(string(hid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM chat_webhooks WHERE id = $1`, string(hid))
	return err
}

func (r *Webhooks) one(ctx context.Context, where string, arg any) (*domain.Webhook, error) {
	rows, _ := r.pool.Query(ctx, `SELECT `+webhookCols+` FROM chat_webhooks WHERE `+where, arg)
	h, err := pgx.CollectExactlyOneRow(rows, scanWebhook)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWebhookNotFound
	}
	return h, err
}

func (r *Webhooks) ByID(ctx context.Context, hid domain.WebhookID) (*domain.Webhook, error) {
	if !id.Valid(string(hid)) {
		return nil, domain.ErrWebhookNotFound
	}
	return r.one(ctx, `id = $1`, string(hid))
}

func (r *Webhooks) BySecret(ctx context.Context, hash string) (*domain.Webhook, error) {
	return r.one(ctx, `secret_hash = $1`, hash)
}

func (r *Webhooks) In(ctx context.Context, conv domain.ConversationID) ([]*domain.Webhook, error) {
	if !id.Valid(string(conv)) {
		return []*domain.Webhook{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+webhookCols+` FROM chat_webhooks WHERE conversation_id = $1 ORDER BY created_at, id`, string(conv))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanWebhook)
}
