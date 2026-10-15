package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type PrefsRepository struct{ pool *pgxpool.Pool }

func NewPrefsRepository(pool *pgxpool.Pool) *PrefsRepository { return &PrefsRepository{pool: pool} }

func (r *PrefsRepository) Prefs(ctx context.Context, user domain.UserID) (domain.Prefs, error) {
	var p domain.Prefs
	err := r.pool.QueryRow(ctx, `SELECT email, telegram, time_zone, email_digest FROM notification_prefs WHERE user_id = $1`, string(user)).
		Scan(&p.Email, &p.Telegram, &p.TimeZone, &p.EmailDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultPrefs(), nil
	}
	return p, err
}

func (r *PrefsRepository) SetPrefs(ctx context.Context, user domain.UserID, p domain.Prefs) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notification_prefs (user_id, email, telegram, time_zone, email_digest) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email, telegram = EXCLUDED.telegram, time_zone = EXCLUDED.time_zone,
		email_digest = EXCLUDED.email_digest`,
		string(user), p.Email, p.Telegram, p.TimeZone, p.EmailDigest)
	return err
}

func (r *PrefsRepository) DigestSubscribers(ctx context.Context) ([]domain.DigestSubscriber, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id::text, email, telegram, time_zone, email_digest, digest_sent_at
		FROM notification_prefs WHERE email_digest ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DigestSubscriber, error) {
		var s domain.DigestSubscriber
		var last *time.Time
		err := row.Scan(&s.User, &s.Prefs.Email, &s.Prefs.Telegram, &s.Prefs.TimeZone, &s.Prefs.EmailDigest, &last)
		if last != nil {
			s.LastSent = last.UTC()
		}
		return s, err
	})
}

func (r *PrefsRepository) MarkDigestSent(ctx context.Context, user domain.UserID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE notification_prefs SET digest_sent_at = $2 WHERE user_id = $1`, string(user), at)
	return err
}
