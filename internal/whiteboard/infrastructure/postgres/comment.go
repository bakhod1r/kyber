package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

// Comments stores element comments (S86).
type Comments struct{ pool *pgxpool.Pool }

func NewComments(pool *pgxpool.Pool) *Comments { return &Comments{pool: pool} }

const commentCols = `id::text, workspace_id::text, board_id::text, conversation_id::text, element_id, author_id::text, body, created_at`

func scanComment(row pgx.CollectableRow) (*domain.Comment, error) {
	var c domain.Comment
	if err := row.Scan(&c.ID, &c.Workspace, &c.Board, &c.Conversation, &c.Element, &c.Author, &c.Body, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, nil
}

func (r *Comments) Add(ctx context.Context, c *domain.Comment) error {
	events := c.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO whiteboard_comments (id, workspace_id, board_id, conversation_id, element_id, author_id, body, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, string(c.ID), string(c.Workspace), string(c.Board), string(c.Conversation), string(c.Element),
			string(c.Author), c.Body, c.CreatedAt); err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

func (r *Comments) Remove(ctx context.Context, cid domain.CommentID) error {
	if !id.Valid(string(cid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM whiteboard_comments WHERE id = $1`, string(cid))
	return err
}

func (r *Comments) ByID(ctx context.Context, cid domain.CommentID) (*domain.Comment, error) {
	if !id.Valid(string(cid)) {
		return nil, domain.ErrCommentMissing
	}
	rows, _ := r.pool.Query(ctx, `SELECT `+commentCols+` FROM whiteboard_comments WHERE id = $1`, string(cid))
	c, err := pgx.CollectExactlyOneRow(rows, scanComment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCommentMissing
	}
	return c, err
}

func (r *Comments) OnBoard(ctx context.Context, b domain.WhiteboardID) ([]*domain.Comment, error) {
	if !id.Valid(string(b)) {
		return []*domain.Comment{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+commentCols+` FROM whiteboard_comments WHERE board_id = $1 ORDER BY created_at, id`, string(b))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanComment)
}
