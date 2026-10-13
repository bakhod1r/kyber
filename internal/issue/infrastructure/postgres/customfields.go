package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type CustomFieldRepository struct{ pool *pgxpool.Pool }

func NewCustomFieldRepository(pool *pgxpool.Pool) *CustomFieldRepository {
	return &CustomFieldRepository{pool: pool}
}

func (r *CustomFieldRepository) SaveField(ctx context.Context, f *domain.CustomField) error {
	s := f.Snapshot()
	opts := s.Options
	if opts == nil {
		opts = []string{}
	}
	var tag pgconn.CommandTag
	var err error
	if s.Version == 0 {
		tag, err = r.pool.Exec(ctx, `INSERT INTO custom_fields (id, workspace_id, project_key, name, type, options, version)
			SELECT $1, $2, $3, $4, $5, $6, 1
			WHERE NOT EXISTS (SELECT 1 FROM custom_fields WHERE workspace_id = $2 AND project_key = $3 AND lower(name) = lower($4))
			ON CONFLICT DO NOTHING`, string(s.ID), tenant.Workspace(ctx), s.Project, s.Name, string(s.Type), opts)
	} else {
		tag, err = r.pool.Exec(ctx, `UPDATE custom_fields SET name = $2, options = $3, version = version + 1
			WHERE id = $1 AND version = $4 AND workspace_id = $5
			  AND NOT EXISTS (SELECT 1 FROM custom_fields o WHERE o.workspace_id = $5 AND o.project_key = custom_fields.project_key
			                  AND o.id <> $1 AND lower(o.name) = lower($2))`,
			string(s.ID), s.Name, opts, s.Version, tenant.Workspace(ctx))
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrFieldConflict
	}
	f.MarkPersisted()
	return nil
}

const fieldCols = `SELECT id::text, project_key, name, type, options, version FROM custom_fields`

func scanField(row pgx.CollectableRow) (*domain.CustomField, error) {
	var s domain.CustomFieldSnapshot
	if err := row.Scan(&s.ID, &s.Project, &s.Name, &s.Type, &s.Options, &s.Version); err != nil {
		return nil, err
	}
	return domain.RehydrateCustomField(s), nil
}

func (r *CustomFieldRepository) FieldByID(ctx context.Context, id domain.FieldID) (*domain.CustomField, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrFieldNotFound
	}
	rows, err := r.pool.Query(ctx, fieldCols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectExactlyOneRow(rows, scanField)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrFieldNotFound
	}
	return f, err
}

func (r *CustomFieldRepository) FieldsOf(ctx context.Context, project string) ([]*domain.CustomField, error) {
	rows, err := r.pool.Query(ctx, fieldCols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY created_at, id`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanField)
}

func (r *CustomFieldRepository) DeleteField(ctx context.Context, id domain.FieldID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrFieldNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM custom_fields WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFieldNotFound
	}
	return nil
}
