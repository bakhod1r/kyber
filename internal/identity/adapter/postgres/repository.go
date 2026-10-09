// Package postgres is the PostgreSQL user and session store.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/identity/domain"
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

func (r *Repository) ByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, userCols+` WHERE id = $1`, string(id)))
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
