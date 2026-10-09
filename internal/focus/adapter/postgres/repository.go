// Package postgres is the PostgreSQL focus-session store.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/focus/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// Create relies on the partial unique index focus_sessions_one_active.
func (r *Repository) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO focus_sessions (id, user_id, workspace_id, issue_key, planned_s, started_at, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.ID, s.User, s.Workspace, s.Issue, int(s.Planned.Seconds()), s.StartedAt, string(s.State))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrAlreadyRunning
	}
	return err
}

func (r *Repository) Save(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `UPDATE focus_sessions SET state = $2, paused_at = $3, paused_s = $4, ended_at = $5, reason = $6
		WHERE id = $1`, s.ID, string(s.State), nullTime(s.PausedAt), s.PausedFor.Seconds(), nullTime(s.EndedAt), s.Reason)
	return err
}

const cols = `id::text, user_id::text, workspace_id::text, issue_key, planned_s, started_at, state, paused_at, paused_s, ended_at, reason`

func scan(row pgx.CollectableRow) (*domain.Session, error) {
	var s domain.Session
	var planned int
	var paused float64
	var pausedAt, endedAt *time.Time
	if err := row.Scan(&s.ID, &s.User, &s.Workspace, &s.Issue, &planned, &s.StartedAt, &s.State, &pausedAt, &paused, &endedAt, &s.Reason); err != nil {
		return nil, err
	}
	s.Planned, s.PausedFor, s.StartedAt = time.Duration(planned)*time.Second, time.Duration(paused*float64(time.Second)), s.StartedAt.UTC()
	if pausedAt != nil {
		s.PausedAt = pausedAt.UTC()
	}
	if endedAt != nil {
		s.EndedAt = endedAt.UTC()
	}
	return &s, nil
}

func (r *Repository) Active(ctx context.Context, user string) (*domain.Session, error) {
	if !id.Valid(user) {
		return nil, domain.ErrSessionNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM focus_sessions WHERE user_id = $1 AND state IN ('running', 'paused')`, user)
	if err != nil {
		return nil, err
	}
	s, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSessionNotFound
	}
	return s, err
}

const historyLimit = 200

func (r *Repository) ForIssue(ctx context.Context, workspace, issue string) ([]*domain.Session, error) {
	if !id.Valid(workspace) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM focus_sessions WHERE workspace_id = $1 AND issue_key = $2
		ORDER BY started_at DESC, id LIMIT $3`, workspace, issue, historyLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) ForUser(ctx context.Context, user string, from, to time.Time) ([]*domain.Session, error) {
	if !id.Valid(user) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM focus_sessions WHERE user_id = $1 AND started_at >= $2 AND started_at < $3
		ORDER BY started_at DESC, id LIMIT $4`, user, from, to, historyLimit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}
