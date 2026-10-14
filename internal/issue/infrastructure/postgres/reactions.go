package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type ReactionRepository struct{ pool *pgxpool.Pool }

func NewReactionRepository(pool *pgxpool.Pool) *ReactionRepository {
	return &ReactionRepository{pool: pool}
}

func (r *ReactionRepository) React(ctx context.Context, re domain.Reaction, at time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO comment_reactions (workspace_id, comment_id, user_id, emoji, reacted_at)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
		tenant.Workspace(ctx), string(re.Comment), string(re.User), string(re.Emoji), at)
	return err
}

func (r *ReactionRepository) Unreact(ctx context.Context, re domain.Reaction) error {
	if !kid.Valid(string(re.Comment)) || !kid.Valid(string(re.User)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM comment_reactions
		WHERE workspace_id = $1 AND comment_id = $2 AND user_id = $3 AND emoji = $4`,
		tenant.Workspace(ctx), string(re.Comment), string(re.User), string(re.Emoji))
	return err
}

func (r *ReactionRepository) ForComments(ctx context.Context, comments []domain.CommentID) ([]domain.Reaction, error) {
	ids := make([]string, 0, len(comments))
	for _, c := range comments {
		if kid.Valid(string(c)) {
			ids = append(ids, string(c))
		}
	}
	rows, err := r.pool.Query(ctx, `SELECT comment_id::text, user_id::text, emoji FROM comment_reactions
		WHERE workspace_id = $1 AND comment_id = ANY($2::uuid[]) ORDER BY reacted_at, comment_id, user_id, emoji`,
		tenant.Workspace(ctx), ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Reaction, error) {
		var c, u, e string
		err := row.Scan(&c, &u, &e)
		return domain.Reaction{Comment: domain.CommentID(c), User: domain.UserID(u), Emoji: domain.Emoji(e)}, err
	})
}
