package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type CommentRepository struct{ pool *pgxpool.Pool }

func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{pool: pool}
}

func (r *CommentRepository) Add(ctx context.Context, c *domain.Comment, events []domain.Event) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO comments (id, issue_id, author_id, body, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			string(c.ID()), string(c.IssueID()), string(c.Author()), c.Body(), c.CreatedAt()); err != nil {
			return err
		}
		return writeOutbox(ctx, tx, events)
	})
}

func (r *CommentRepository) ListByIssue(ctx context.Context, issueID domain.IssueID) ([]*domain.Comment, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, issue_id::text, author_id::text, body, created_at
		FROM comments WHERE issue_id = $1 ORDER BY created_at, id`, string(issueID))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Comment, error) {
		var id, issue, author, body string
		var createdAt time.Time
		if err := row.Scan(&id, &issue, &author, &body, &createdAt); err != nil {
			return nil, err
		}
		return domain.RehydrateComment(domain.CommentID(id), domain.IssueID(issue), domain.UserID(author), body, createdAt), nil
	})
}
