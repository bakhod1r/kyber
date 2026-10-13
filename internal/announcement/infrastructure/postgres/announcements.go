// Package postgres stores announcements in PostgreSQL, writing their events to the outbox.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, a *domain.Announcement) error {
	events := a.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO announcements (id, workspace_id, project_key, title, body, author_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, string(a.ID), a.Workspace, a.Project, a.Title, a.Body, a.Author, a.At); err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

const cols = `SELECT id::text, workspace_id::text, project_key, title, body, author_id::text, created_at FROM announcements`

func scan(row pgx.CollectableRow) (*domain.Announcement, error) {
	var a domain.Announcement
	err := row.Scan(&a.ID, &a.Workspace, &a.Project, &a.Title, &a.Body, &a.Author, &a.At)
	a.At = a.At.UTC()
	return &a, err
}

func (r *Repository) ByID(ctx context.Context, id domain.AnnouncementID) (*domain.Announcement, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrAnnouncementNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	a, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAnnouncementNotFound
	}
	return a, err
}

func (r *Repository) For(ctx context.Context, project string, limit int) ([]*domain.Announcement, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY created_at DESC, id LIMIT $3`,
		tenant.Workspace(ctx), project, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}
