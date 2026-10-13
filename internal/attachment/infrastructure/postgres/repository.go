// Package postgres stores attachment records in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, a *domain.Attachment) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO attachments (id, workspace_id, issue_key, name, content_type, size, sha256, uploaded_by, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		string(a.ID), tenant.Workspace(ctx), a.Issue, a.Name, a.ContentType, a.Size, a.SHA256, a.UploadedBy, a.UploadedAt)
	return err
}

const cols = `SELECT id::text, issue_key, name, content_type, size, sha256, uploaded_by::text, uploaded_at FROM attachments`

func scan(row pgx.CollectableRow) (*domain.Attachment, error) {
	var a domain.Attachment
	err := row.Scan(&a.ID, &a.Issue, &a.Name, &a.ContentType, &a.Size, &a.SHA256, &a.UploadedBy, &a.UploadedAt)
	a.UploadedAt = a.UploadedAt.UTC()
	return &a, err
}

func (r *Repository) ByID(ctx context.Context, id domain.ID) (*domain.Attachment, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrAttachmentNotFound
	}
	rows, _ := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	a, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAttachmentNotFound
	}
	return a, err
}

func (r *Repository) ForIssue(ctx context.Context, issue string) ([]*domain.Attachment, error) {
	rows, _ := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND issue_key = $2 ORDER BY uploaded_at, id`, tenant.Workspace(ctx), issue)
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Delete(ctx context.Context, id domain.ID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrAttachmentNotFound
	}
	return err
}
