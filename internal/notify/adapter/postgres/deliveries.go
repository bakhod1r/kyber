package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type DeliveryRepository struct{ pool *pgxpool.Pool }

func NewDeliveryRepository(pool *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{pool: pool}
}

func (r *DeliveryRepository) Enqueue(ctx context.Context, d domain.Delivery) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notification_deliveries
		(id, workspace_id, channel, recipient, subject, body, key, status, attempts, next_at, last_error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (key) DO NOTHING`,
		d.ID, d.Workspace, string(d.Channel), d.To, d.Subject, d.Body, d.Key, string(d.Status), d.Attempts, d.NextAt, d.LastError, d.CreatedAt)
	return err
}

// Claim leases due deliveries atomically; SKIP LOCKED lets several dispatchers run side by side.
func (r *DeliveryRepository) Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	rows, err := r.pool.Query(ctx, `UPDATE notification_deliveries SET next_at = $2
		WHERE id IN (SELECT id FROM notification_deliveries WHERE status = 'pending' AND next_at <= $1
			ORDER BY next_at, created_at, id LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING id::text, workspace_id::text, channel, recipient, subject, body, key, status, attempts, last_error, created_at, next_at`,
		now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Delivery, error) {
		var d domain.Delivery
		var leased time.Time
		err := row.Scan(&d.ID, &d.Workspace, &d.Channel, &d.To, &d.Subject, &d.Body, &d.Key, &d.Status, &d.Attempts, &d.LastError, &d.CreatedAt, &leased)
		d.NextAt = now // as due; Save stores the real next attempt
		d.CreatedAt = d.CreatedAt.UTC()
		return d, err
	})
	// RETURNING has no ORDER BY: restore oldest-first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.Before(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, err
}

func (r *DeliveryRepository) Save(ctx context.Context, d domain.Delivery) error {
	var sent *time.Time
	if !d.SentAt.IsZero() {
		sent = &d.SentAt
	}
	_, err := r.pool.Exec(ctx, `UPDATE notification_deliveries SET status = $2, attempts = $3, next_at = $4,
		last_error = $5, sent_at = $6 WHERE id = $1`, d.ID, string(d.Status), d.Attempts, d.NextAt, d.LastError, sent)
	return err
}
