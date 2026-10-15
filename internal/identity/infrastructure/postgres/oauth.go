package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/bakhod1r/kyber/internal/identity/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

func (r *Repository) CreateClient(ctx context.Context, c *domain.OAuthClient) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO oauth_clients (id, name, redirect_uris, created_at) VALUES ($1, $2, $3, $4)`,
		string(c.ID), c.Name, c.RedirectURIs, c.CreatedAt)
	return err
}

func (r *Repository) Client(ctx context.Context, id domain.ClientID) (*domain.OAuthClient, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrOAuthClientNotFound
	}
	var c domain.OAuthClient
	err := r.pool.QueryRow(ctx, `SELECT id::text, name, redirect_uris, created_at FROM oauth_clients WHERE id = $1`, string(id)).
		Scan(&c.ID, &c.Name, &c.RedirectURIs, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOAuthClientNotFound
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, err
}

func (r *Repository) SaveCode(ctx context.Context, c *domain.AuthCode) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, challenge, expires_at, auth_method)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, c.Hash, string(c.Client), string(c.User), c.RedirectURI, c.Challenge, c.ExpiresAt, c.Method)
	return err
}

// TakeCode deletes and returns the code in one statement, so two exchanges cannot both win.
func (r *Repository) TakeCode(ctx context.Context, hash []byte) (*domain.AuthCode, error) {
	var c domain.AuthCode
	err := r.pool.QueryRow(ctx, `DELETE FROM oauth_codes WHERE code_hash = $1
		RETURNING code_hash, client_id::text, user_id::text, redirect_uri, challenge, expires_at, auth_method`, hash).
		Scan(&c.Hash, &c.Client, &c.User, &c.RedirectURI, &c.Challenge, &c.ExpiresAt, &c.Method)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAuthCodeNotFound
	}
	c.ExpiresAt = c.ExpiresAt.UTC()
	return &c, err
}

var _ domain.OAuthStore = (*Repository)(nil)
