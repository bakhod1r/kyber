// Package postgres stores SSO connections in PostgreSQL (hand-written pgx, ADR-0002). Client
// secrets are sealed (AES-256-GCM) before they reach the database.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/seal"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

type Store struct {
	pool *pgxpool.Pool
	box  *seal.Box // nil: no KYBER_ENCRYPTION_KEY, so nothing can be stored or read
}

func New(pool *pgxpool.Pool, box *seal.Box) *Store { return &Store{pool: pool, box: box} }

const columns = `id::text, kind, name, issuer, client_id, client_secret, groups_claim, COALESCE(workspace_id::text, ''), enabled, created_at`

func (s *Store) scan(r pgx.Row) (domain.Connection, error) {
	var c domain.Connection
	var sealed []byte
	err := r.Scan(&c.ID, &c.Kind, &c.Name, &c.Issuer, &c.ClientID, &sealed, &c.GroupsClaim, &c.Workspace, &c.Enabled, &c.CreatedAt)
	if err != nil {
		return c, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	if s.box == nil {
		return c, domain.ErrNoEncryptionKey
	}
	c.ClientSecret, err = s.box.Open(sealed)
	return c, err
}

func (s *Store) All(ctx context.Context) ([]domain.Connection, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM sso_connections ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Connection, error) { return s.scan(r) })
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Domains, err = s.domains(ctx, all[i].ID); err != nil {
			return nil, err
		}
	}
	return all, nil
}

func (s *Store) domains(ctx context.Context, id domain.ConnectionID) ([]domain.Domain, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, token, verified FROM sso_connection_domains WHERE connection_id = $1 ORDER BY name`, string(id))
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domain.Domain])
	if len(list) == 0 {
		list = nil
	}
	return list, err
}

func (s *Store) ByDomain(ctx context.Context, name string) (*domain.Connection, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT connection_id::text FROM sso_connection_domains WHERE name = lower($1)`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrConnectionNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.ByID(ctx, domain.ConnectionID(id))
}

func (s *Store) ByID(ctx context.Context, id domain.ConnectionID) (*domain.Connection, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrConnectionNotFound
	}
	c, err := s.scan(s.pool.QueryRow(ctx, `SELECT `+columns+` FROM sso_connections WHERE id = $1`, string(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrConnectionNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.Domains, err = s.domains(ctx, id); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) Save(ctx context.Context, c *domain.Connection) error {
	if s.box == nil {
		return domain.ErrNoEncryptionKey
	}
	sealed := s.box.Seal(c.ClientSecret)
	var ws *string
	if c.Workspace != "" {
		ws = &c.Workspace
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO sso_connections (id, kind, name, issuer, client_id, client_secret, groups_claim, workspace_id, enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET name = $3, issuer = $4, client_id = $5, client_secret = $6, groups_claim = $7, enabled = $9`,
		string(c.ID), string(c.Kind), c.Name, c.Issuer, c.ClientID, sealed, c.GroupsClaim, ws, c.Enabled, c.CreatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sso_connection_domains WHERE connection_id = $1`, string(c.ID)); err != nil {
		return err
	}
	for _, d := range c.Domains {
		_, err := tx.Exec(ctx, `INSERT INTO sso_connection_domains (name, connection_id, token, verified) VALUES ($1, $2, $3, $4)`,
			d.Name, string(c.ID), d.Token, d.Verified)
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDomainTaken
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) Delete(ctx context.Context, id domain.ConnectionID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrConnectionNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sso_connections WHERE id = $1`, string(id))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrConnectionNotFound
	}
	return err
}
