// Package postgres is the PostgreSQL sprint repository.
package postgres

import (
	"context"
	"errors"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, s *domain.Sprint, events []domain.Event) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var tag pgconn.CommandTag
		var err error
		if s.Version() == 0 {
			tag, err = tx.Exec(ctx, `INSERT INTO sprints (id, project_key, name, goal, state, started_at, ends_at, completed_at, version, workspace_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9) ON CONFLICT (id) DO NOTHING`,
				string(s.ID()), s.Project(), s.Name(), s.Goal(), string(s.State()), nullTime(s.StartedAt()),
				nullTime(s.EndsAt()), nullTime(s.CompletedAt()), tenant.Workspace(ctx))
		} else {
			tag, err = tx.Exec(ctx, `UPDATE sprints SET name = $2, goal = $3, state = $4, started_at = $5,
				ends_at = $6, completed_at = $7, version = version + 1 WHERE id = $1 AND version = $8 AND workspace_id = $9`,
				string(s.ID()), s.Name(), s.Goal(), string(s.State()), nullTime(s.StartedAt()), nullTime(s.EndsAt()),
				nullTime(s.CompletedAt()), s.Version(), tenant.Workspace(ctx))
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrSprintConflict
		}
		return db.WriteOutbox(ctx, tx, events)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "sprints_one_active" {
		return domain.ErrAnotherSprintLive
	}
	if err != nil {
		return err
	}
	s.MarkPersisted()
	return nil
}

const cols = `SELECT id::text, project_key, name, goal, state, started_at, ends_at, completed_at, version FROM sprints`

func (r *Repository) ByID(ctx context.Context, sid domain.SprintID) (*domain.Sprint, error) {
	if !id.Valid(string(sid)) { // never let client input reach a uuid cast
		return nil, domain.ErrSprintNotFound
	}
	s, err := scan(r.pool.QueryRow(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(sid), tenant.Workspace(ctx)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSprintNotFound
	}
	return s, err
}

func (r *Repository) ListByProject(ctx context.Context, project string) ([]*domain.Sprint, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE project_key = $1 AND workspace_id = $2
		ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'planned' THEN 1 ELSE 2 END,
			CASE WHEN state = 'closed' THEN NULL ELSE created_at END ASC,
			completed_at DESC NULLS LAST, id`, project, tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Sprint, error) { return scan(row) })
}

func scan(row pgx.Row) (*domain.Sprint, error) {
	var s domain.Snapshot
	var started, ends, completed *time.Time
	if err := row.Scan(&s.ID, &s.Project, &s.Name, &s.Goal, &s.State, &started, &ends, &completed, &s.Version); err != nil {
		return nil, err
	}
	if ends != nil {
		s.EndsAt = *ends
	}
	if started != nil {
		s.StartedAt = *started
	}
	if completed != nil {
		s.CompletedAt = *completed
	}
	return domain.Rehydrate(s), nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
