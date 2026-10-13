package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type PrefsRepository struct{ pool *pgxpool.Pool }

func NewPrefsRepository(pool *pgxpool.Pool) *PrefsRepository { return &PrefsRepository{pool: pool} }

func (r *PrefsRepository) Prefs(ctx context.Context, user domain.UserID) (domain.Prefs, error) {
	var p domain.Prefs
	err := r.pool.QueryRow(ctx, `SELECT email, telegram FROM notification_prefs WHERE user_id = $1`, string(user)).Scan(&p.Email, &p.Telegram)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultPrefs(), nil
	}
	return p, err
}

func (r *PrefsRepository) SetPrefs(ctx context.Context, user domain.UserID, p domain.Prefs) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notification_prefs (user_id, email, telegram) VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email, telegram = EXCLUDED.telegram`, string(user), p.Email, p.Telegram)
	return err
}
