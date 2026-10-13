package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type VersionRepository struct{ pool *pgxpool.Pool }

func NewVersionRepository(pool *pgxpool.Pool) *VersionRepository {
	return &VersionRepository{pool: pool}
}

func (r *VersionRepository) SaveVersion(ctx context.Context, v *domain.Version) error {
	s := v.Snapshot()
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var tag pgconn.CommandTag
		var err error
		if s.Version == 0 {
			tag, err = tx.Exec(ctx, `INSERT INTO versions (id, workspace_id, project_key, name, description, release_date, state, released_at, version)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1) ON CONFLICT (id) DO NOTHING`,
				string(s.ID), tenant.Workspace(ctx), s.Project, s.Name, s.Description, nullTime(s.ReleaseDate), string(s.State), nullTime(s.ReleasedAt))
		} else {
			tag, err = tx.Exec(ctx, `UPDATE versions SET name = $2, description = $3, release_date = $4, state = $5, released_at = $6,
				version = version + 1 WHERE id = $1 AND version = $7 AND workspace_id = $8`,
				string(s.ID), s.Name, s.Description, nullTime(s.ReleaseDate), string(s.State), nullTime(s.ReleasedAt), s.Version, tenant.Workspace(ctx))
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrVersionConflict
		}
		return db.WriteOutbox(ctx, tx, v.PullEvents())
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrVersionTaken
	}
	if err != nil {
		return err
	}
	v.MarkPersisted()
	return nil
}

const versionCols = `SELECT id::text, project_key, name, description, release_date, state, released_at, version FROM versions`

func scanVersion(row pgx.CollectableRow) (*domain.Version, error) {
	var s domain.VersionSnapshot
	var date, released *time.Time
	if err := row.Scan(&s.ID, &s.Project, &s.Name, &s.Description, &date, &s.State, &released, &s.Version); err != nil {
		return nil, err
	}
	if date != nil {
		s.ReleaseDate = date.UTC()
	}
	if released != nil {
		s.ReleasedAt = released.UTC()
	}
	return domain.RehydrateVersion(s), nil
}

func (r *VersionRepository) VersionByID(ctx context.Context, id domain.VersionID) (*domain.Version, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrVersionNotFound
	}
	rows, err := r.pool.Query(ctx, versionCols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	v, err := pgx.CollectExactlyOneRow(rows, scanVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrVersionNotFound
	}
	return v, err
}

func (r *VersionRepository) VersionsOf(ctx context.Context, project string) ([]*domain.Version, error) {
	rows, err := r.pool.Query(ctx, versionCols+` WHERE workspace_id = $1 AND project_key = $2
		ORDER BY release_date NULLS LAST, name COLLATE "C"`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanVersion)
}

// DeleteVersion removes the version and writes its VersionRemoved event in one transaction.
func (r *VersionRepository) DeleteVersion(ctx context.Context, v *domain.Version) error {
	if !kid.Valid(string(v.ID())) {
		return domain.ErrVersionNotFound
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM versions WHERE id = $1 AND workspace_id = $2`, string(v.ID()), tenant.Workspace(ctx))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrVersionNotFound
		}
		return db.WriteOutbox(ctx, tx, v.PullEvents())
	})
}
