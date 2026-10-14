package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/docs/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Comments stores paragraph comments (S86).
type Comments struct{ pool *pgxpool.Pool }

func NewComments(pool *pgxpool.Pool) *Comments { return &Comments{pool: pool} }

const commentCols = `SELECT id::text, page_id::text, project_key, paragraph, quote, author_id::text, body, created_at FROM doc_comments`

func scanComment(row pgx.CollectableRow) (*domain.Comment, error) {
	var c domain.Comment
	if err := row.Scan(&c.ID, &c.Page, &c.Project, &c.Paragraph, &c.Quote, &c.Author, &c.Body, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, nil
}

func (r *Comments) Add(ctx context.Context, c *domain.Comment) error {
	events := c.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO doc_comments (id, workspace_id, page_id, project_key, paragraph, quote, author_id, body, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, string(c.ID), tenant.Workspace(ctx), string(c.Page), c.Project, c.Paragraph,
			c.Quote, c.Author, c.Body, c.CreatedAt); err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

func (r *Comments) Remove(ctx context.Context, id domain.CommentID) error {
	if !kid.Valid(string(id)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM doc_comments WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	return err
}

func (r *Comments) ByID(ctx context.Context, id domain.CommentID) (*domain.Comment, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrCommentMissing
	}
	rows, _ := r.pool.Query(ctx, commentCols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	c, err := pgx.CollectExactlyOneRow(rows, scanComment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCommentMissing
	}
	return c, err
}

func (r *Comments) OnPage(ctx context.Context, p domain.PageID) ([]*domain.Comment, error) {
	if !kid.Valid(string(p)) {
		return []*domain.Comment{}, nil
	}
	rows, err := r.pool.Query(ctx, commentCols+` WHERE page_id = $1 AND workspace_id = $2 ORDER BY created_at, id`, string(p), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanComment)
}
