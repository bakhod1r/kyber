// Package postgres stores webhooks, their deliveries and development links in PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/webhook/domain"
)

type Webhooks struct{ pool *pgxpool.Pool }

func NewWebhooks(pool *pgxpool.Pool) *Webhooks { return &Webhooks{pool: pool} }

func (r *Webhooks) Save(ctx context.Context, w *domain.Webhook) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO webhooks (id, workspace_id, project_key, url, secret, events, active, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET url = EXCLUDED.url, events = EXCLUDED.events, active = EXCLUDED.active
		WHERE webhooks.workspace_id = EXCLUDED.workspace_id`,
		string(w.ID), tenant.Workspace(ctx), w.Project, w.URL, w.Secret, w.Events, w.Active, w.CreatedBy, w.CreatedAt)
	return err
}

const webhookCols = `SELECT id::text, project_key, url, secret, events, active, created_by::text, created_at FROM webhooks`

func scanWebhook(row pgx.CollectableRow) (*domain.Webhook, error) {
	var w domain.Webhook
	err := row.Scan(&w.ID, &w.Project, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedBy, &w.CreatedAt)
	w.CreatedAt = w.CreatedAt.UTC()
	return &w, err
}

func (r *Webhooks) ByID(ctx context.Context, id domain.ID) (*domain.Webhook, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrWebhookNotFound
	}
	rows, _ := r.pool.Query(ctx, webhookCols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	w, err := pgx.CollectExactlyOneRow(rows, scanWebhook)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWebhookNotFound
	}
	return w, err
}

func (r *Webhooks) ForProject(ctx context.Context, project string) ([]*domain.Webhook, error) {
	rows, _ := r.pool.Query(ctx, webhookCols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY created_at, id`,
		tenant.Workspace(ctx), project)
	return pgx.CollectRows(rows, scanWebhook)
}

func (r *Webhooks) Delete(ctx context.Context, id domain.ID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrWebhookNotFound
	}
	return err
}

type Deliveries struct{ pool *pgxpool.Pool }

func NewDeliveries(pool *pgxpool.Pool) *Deliveries { return &Deliveries{pool: pool} }

func (r *Deliveries) Enqueue(ctx context.Context, d domain.Delivery) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO webhook_deliveries (id, workspace_id, webhook_id, event, key, body, status, attempts, next_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (key) DO NOTHING`,
		d.ID, d.Workspace, string(d.Webhook), d.Event, d.Key, d.Body, string(d.Status), d.Attempts, d.NextAt, d.CreatedAt)
	return err
}

const deliveryCols = `id::text, workspace_id::text, webhook_id::text, event, key, body, status, attempts, next_at, last_status, last_error, created_at, delivered_at`

func scanDelivery(row pgx.CollectableRow) (domain.Delivery, error) {
	var d domain.Delivery
	var delivered *time.Time
	err := row.Scan(&d.ID, &d.Workspace, &d.Webhook, &d.Event, &d.Key, &d.Body, &d.Status, &d.Attempts, &d.NextAt, &d.LastStatus,
		&d.LastError, &d.CreatedAt, &delivered)
	d.NextAt, d.CreatedAt = d.NextAt.UTC(), d.CreatedAt.UTC()
	if delivered != nil {
		d.DeliveredAt = delivered.UTC()
	}
	return d, err
}

// Claim leases due deliveries atomically; SKIP LOCKED lets several dispatchers run side by side.
func (r *Deliveries) Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.Delivery, error) {
	rows, _ := r.pool.Query(ctx, `UPDATE webhook_deliveries SET next_at = $2
		WHERE id IN (SELECT id FROM webhook_deliveries WHERE status = 'pending' AND next_at <= $1
			ORDER BY next_at, created_at, id LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING `+deliveryCols, now, now.Add(lease), limit)
	out, err := pgx.CollectRows(rows, scanDelivery)
	for i := range out {
		out[i].NextAt = now // as due; Save stores the real next attempt
	}
	// RETURNING has no ORDER BY: restore oldest-first.
	slices.SortStableFunc(out, func(a, b domain.Delivery) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out, err
}

func (r *Deliveries) Save(ctx context.Context, d domain.Delivery) error {
	var delivered *time.Time
	if !d.DeliveredAt.IsZero() {
		delivered = &d.DeliveredAt
	}
	_, err := r.pool.Exec(ctx, `UPDATE webhook_deliveries SET status = $2, attempts = $3, next_at = $4, last_status = $5,
		last_error = $6, delivered_at = $7 WHERE id = $1`, d.ID, string(d.Status), d.Attempts, d.NextAt, d.LastStatus, d.LastError, delivered)
	return err
}

func (r *Deliveries) Recent(ctx context.Context, webhook domain.ID, limit int) ([]domain.Delivery, error) {
	rows, _ := r.pool.Query(ctx, `SELECT `+deliveryCols+` FROM webhook_deliveries WHERE workspace_id = $1 AND webhook_id = $2
		ORDER BY created_at DESC, id DESC LIMIT $3`, tenant.Workspace(ctx), string(webhook), limit)
	return pgx.CollectRows(rows, scanDelivery)
}

type Links struct{ pool *pgxpool.Pool }

func NewLinks(pool *pgxpool.Pool) *Links { return &Links{pool: pool} }

func (r *Links) Save(ctx context.Context, l domain.Link) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO development_links (workspace_id, issue_key, kind, repo, ref, url, title, state, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (workspace_id, issue_key, kind, repo, ref) DO UPDATE
		SET url = EXCLUDED.url, title = EXCLUDED.title, state = EXCLUDED.state, updated_at = EXCLUDED.updated_at`,
		tenant.Workspace(ctx), l.Issue, string(l.Kind), l.Repo, l.Ref, l.URL, l.Title, l.State, l.UpdatedAt)
	return err
}

func (r *Links) ForIssue(ctx context.Context, issue string) ([]domain.Link, error) {
	rows, _ := r.pool.Query(ctx, `SELECT issue_key, kind, repo, ref, url, title, state, updated_at FROM development_links
		WHERE workspace_id = $1 AND issue_key = $2 ORDER BY updated_at DESC, kind, ref`, tenant.Workspace(ctx), issue)
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Link, error) {
		var l domain.Link
		err := row.Scan(&l.Issue, &l.Kind, &l.Repo, &l.Ref, &l.URL, &l.Title, &l.State, &l.UpdatedAt)
		l.UpdatedAt = l.UpdatedAt.UTC()
		return l, err
	})
}
