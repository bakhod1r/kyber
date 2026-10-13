package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bakhod1r/kyber/internal/identity/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

func (r *Repository) CreateToken(ctx context.Context, t *domain.PersonalToken) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO personal_tokens (id, user_id, name, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, string(t.ID), string(t.UserID), t.Name, t.Hash, t.CreatedAt, t.ExpiresAt)
	return err
}

const tokenCols = `SELECT id::text, user_id::text, name, token_hash, created_at, expires_at, last_used_at FROM personal_tokens`

func scanToken(row pgx.CollectableRow) (*domain.PersonalToken, error) {
	var t domain.PersonalToken
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Hash, &t.CreatedAt, &t.ExpiresAt, &t.LastUsedAt)
	t.CreatedAt = t.CreatedAt.UTC()
	for _, p := range []*time.Time{t.ExpiresAt, t.LastUsedAt} {
		if p != nil {
			*p = p.UTC()
		}
	}
	return &t, err
}

func (r *Repository) TokenByHash(ctx context.Context, hash []byte) (*domain.PersonalToken, error) {
	rows, _ := r.pool.Query(ctx, tokenCols+` WHERE token_hash = $1`, hash)
	t, err := pgx.CollectExactlyOneRow(rows, scanToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTokenNotFound
	}
	return t, err
}

func (r *Repository) TokensOf(ctx context.Context, user domain.UserID) ([]*domain.PersonalToken, error) {
	rows, _ := r.pool.Query(ctx, tokenCols+` WHERE user_id = $1 ORDER BY created_at DESC, id`, string(user))
	return pgx.CollectRows(rows, scanToken)
}

func (r *Repository) DeleteToken(ctx context.Context, user domain.UserID, id domain.TokenID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrTokenNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM personal_tokens WHERE id = $1 AND user_id = $2`, string(id), string(user))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrTokenNotFound
	}
	return err
}

func (r *Repository) TouchToken(ctx context.Context, id domain.TokenID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE personal_tokens SET last_used_at = $2 WHERE id = $1`, string(id), at)
	return err
}
