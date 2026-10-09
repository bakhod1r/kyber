// Package postgres is the PostgreSQL user and session store.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, $3, $4)`,
		string(u.ID()), u.Email().String(), u.Name(), u.PasswordHash())
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrEmailTaken
	}
	return err
}

const userCols = `SELECT id::text, email, name, password_hash FROM users`

func (r *Repository) ByEmail(ctx context.Context, e domain.Email) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, userCols+` WHERE email = $1`, e.String()))
}

func (r *Repository) ByID(ctx context.Context, uid domain.UserID) (*domain.User, error) {
	if !id.Valid(string(uid)) { // e.g. legacy events without an actor
		return nil, domain.ErrUserNotFound
	}
	return scanUser(r.pool.QueryRow(ctx, userCols+` WHERE id = $1`, string(uid)))
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var id, email, name, hash string
	if err := row.Scan(&id, &email, &name, &hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return domain.NewUser(domain.UserID(id), domain.Email(email), name, hash)
}

func (r *Repository) CreateSession(ctx context.Context, s domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		s.TokenHash, string(s.UserID), s.ExpiresAt)
	return err
}

func (r *Repository) SessionByHash(ctx context.Context, hash []byte) (domain.Session, error) {
	var s domain.Session
	var uid string
	err := r.pool.QueryRow(ctx, `SELECT token_hash, user_id::text, expires_at FROM sessions WHERE token_hash = $1`, hash).
		Scan(&s.TokenHash, &uid, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	s.UserID = domain.UserID(uid)
	return s, err
}

func (r *Repository) DeleteSession(ctx context.Context, hash []byte) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
	return err
}

func (r *Repository) DeleteExpiredSessions(ctx context.Context, now time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	return int(tag.RowsAffected()), err
}

func (r *Repository) LinkedUser(ctx context.Context, p domain.Provider, subject string) (domain.UserID, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT user_id::text FROM external_identities WHERE provider = $1 AND subject = $2`,
		string(p), subject).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotLinked
	}
	return domain.UserID(id), err
}

// Link is idempotent; an identity already linked is never moved to another user.
func (r *Repository) Link(ctx context.Context, id domain.ExternalIdentity) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO external_identities (provider, subject, user_id) VALUES ($1, $2, $3)
		ON CONFLICT (provider, subject) DO NOTHING`, string(id.Provider), id.Subject, string(id.UserID))
	return err
}

const otpColumns = `id::text, nonce, expires_at, telegram_id, name, code_hash, attempts, used`

func scanOTP(row pgx.Row) (*domain.OTPChallenge, error) {
	var c domain.OTPChallenge
	err := row.Scan(&c.ID, &c.Nonce, &c.ExpiresAt, &c.TelegramID, &c.Name, &c.CodeHash, &c.Attempts, &c.Used)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOTPNotFound
	}
	return &c, err
}

func (r *Repository) CreateOTP(ctx context.Context, c *domain.OTPChallenge) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO otp_challenges (id, nonce, expires_at) VALUES ($1, $2, $3)`, c.ID, c.Nonce, c.ExpiresAt)
	return err
}

func (r *Repository) OTPByID(ctx context.Context, oid string) (*domain.OTPChallenge, error) {
	if !id.Valid(oid) {
		return nil, domain.ErrOTPNotFound
	}
	return scanOTP(r.pool.QueryRow(ctx, `SELECT `+otpColumns+` FROM otp_challenges WHERE id = $1`, oid))
}

func (r *Repository) OTPByNonce(ctx context.Context, nonce string) (*domain.OTPChallenge, error) {
	return scanOTP(r.pool.QueryRow(ctx, `SELECT `+otpColumns+` FROM otp_challenges WHERE nonce = $1`, nonce))
}

// UpdateOTP locks the row so concurrent guesses are counted one by one; the changes made by
// fn are committed even when fn fails (a wrong code must still count).
func (r *Repository) UpdateOTP(ctx context.Context, oid string, fn func(*domain.OTPChallenge) error) error {
	if !id.Valid(oid) {
		return domain.ErrOTPNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	c, err := scanOTP(tx.QueryRow(ctx, `SELECT `+otpColumns+` FROM otp_challenges WHERE id = $1 FOR UPDATE`, oid))
	if err != nil {
		return err
	}
	fnErr := fn(c)
	if _, err := tx.Exec(ctx, `UPDATE otp_challenges SET telegram_id = $2, name = $3, code_hash = $4, attempts = $5, used = $6 WHERE id = $1`,
		oid, c.TelegramID, c.Name, c.CodeHash, c.Attempts, c.Used); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return fnErr
}
