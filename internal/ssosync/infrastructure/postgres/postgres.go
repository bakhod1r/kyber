// Package postgres stores ssosync state in PostgreSQL (hand-written pgx, ADR-0002).
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) All(ctx context.Context) ([]domain.Mapping, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, group_name, scope, workspace_id::text, project_key, role,
		COALESCE(created_by::text, ''), created_at FROM sso_group_mappings ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Mapping, error) {
		var m domain.Mapping
		err := r.Scan(&m.ID, &m.Group, &m.Target.Scope, &m.Target.Workspace, &m.Target.Project, &m.Role, &m.CreatedBy, &m.CreatedAt)
		m.CreatedAt = m.CreatedAt.UTC()
		return m, err
	})
}

func (s *Store) Create(ctx context.Context, m *domain.Mapping) error {
	var by *string
	if kid.Valid(m.CreatedBy) {
		by = &m.CreatedBy
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO sso_group_mappings (id, group_name, scope, workspace_id, project_key, role, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, string(m.ID), m.Group, string(m.Target.Scope), m.Target.Workspace, m.Target.Project, m.Role, by, m.CreatedAt)
	return err
}

func (s *Store) Delete(ctx context.Context, id domain.MappingID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrMappingNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sso_group_mappings WHERE id = $1`, string(id))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrMappingNotFound
	}
	return err
}

func (s *Store) Of(ctx context.Context, user string) ([]domain.Grant, error) {
	if !kid.Valid(user) {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT user_id::text, scope, workspace_id::text, project_key, role FROM sso_grants WHERE user_id = $1`, user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Grant, error) {
		var g domain.Grant
		return g, r.Scan(&g.User, &g.Target.Scope, &g.Target.Workspace, &g.Target.Project, &g.Role)
	})
}

func (s *Store) Save(ctx context.Context, g domain.Grant) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sso_grants (user_id, scope, workspace_id, project_key, role) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, scope, workspace_id, project_key) DO UPDATE SET role = EXCLUDED.role`,
		g.User, string(g.Target.Scope), g.Target.Workspace, g.Target.Project, g.Role)
	return err
}

func (s *Store) Remove(ctx context.Context, user string, t domain.Target) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sso_grants WHERE user_id = $1 AND scope = $2 AND workspace_id = $3 AND project_key = $4`,
		user, string(t.Scope), t.Workspace, t.Project)
	return err
}

var (
	_ domain.Mappings = (*Store)(nil)
	_ domain.Grants   = (*Store)(nil)
)
