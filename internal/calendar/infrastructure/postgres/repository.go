// Package postgres is the PostgreSQL meeting store; it writes meeting events to the outbox.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, m *domain.Meeting) error {
	events := m.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO meetings (id, workspace_id, title, starts_at, ends_at, organizer_id, scope, project_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, string(m.ID), string(m.Workspace), m.Title, m.Start, m.End, string(m.Organizer),
			string(m.Audience.Scope), m.Audience.Project); err != nil {
			return err
		}
		for _, a := range m.Attendees {
			if _, err := tx.Exec(ctx, `INSERT INTO meeting_attendees (meeting_id, user_id) VALUES ($1, $2)`, string(m.ID), string(a)); err != nil {
				return err
			}
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

func (r *Repository) Cancel(ctx context.Context, m *domain.Meeting) error {
	if !id.Valid(string(m.ID)) {
		return domain.ErrMeetingNotFound
	}
	events := m.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM meetings WHERE id = $1`, string(m.ID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrMeetingNotFound
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

const cols = `m.id::text, m.workspace_id::text, m.title, m.starts_at, m.ends_at, m.organizer_id::text, m.scope, m.project_key,
	m.reminded_at IS NOT NULL,
	array(SELECT a.user_id::text FROM meeting_attendees a WHERE a.meeting_id = m.id ORDER BY a.user_id::text),
	array(SELECT a.response FROM meeting_attendees a WHERE a.meeting_id = m.id ORDER BY a.user_id::text)`

func scan(row pgx.CollectableRow) (*domain.Meeting, error) {
	m := domain.Meeting{Responses: map[domain.UserID]domain.Response{}}
	var attendees, responses []string
	if err := row.Scan(&m.ID, &m.Workspace, &m.Title, &m.Start, &m.End, &m.Organizer, &m.Audience.Scope, &m.Audience.Project,
		&m.Reminded, &attendees, &responses); err != nil {
		return nil, err
	}
	for i, a := range attendees {
		m.Attendees = append(m.Attendees, domain.UserID(a))
		if responses[i] != "" {
			m.Responses[domain.UserID(a)] = domain.Response(responses[i])
		}
	}
	m.Start, m.End = m.Start.UTC(), m.End.UTC()
	return &m, nil
}

func (r *Repository) ByID(ctx context.Context, mid domain.MeetingID) (*domain.Meeting, error) {
	if !id.Valid(string(mid)) {
		return nil, domain.ErrMeetingNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM meetings m WHERE m.id = $1`, string(mid))
	if err != nil {
		return nil, err
	}
	m, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMeetingNotFound
	}
	return m, err
}

func (r *Repository) ForUser(ctx context.Context, u domain.UserID, from, to time.Time) ([]*domain.Meeting, error) {
	if !id.Valid(string(u)) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM meetings m
		JOIN meeting_attendees me ON me.meeting_id = m.id AND me.user_id = $1
		WHERE m.starts_at < $3 AND m.ends_at > $2 ORDER BY m.starts_at, m.id`, string(u), from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Respond(ctx context.Context, m *domain.Meeting, u domain.UserID) error {
	tag, err := r.pool.Exec(ctx, `UPDATE meeting_attendees SET response = $3 WHERE meeting_id = $1 AND user_id = $2`,
		string(m.ID), string(u), string(m.Responses[u]))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrMeetingNotFound
	}
	return err
}

func (r *Repository) DueForReminder(ctx context.Context, from, to time.Time) ([]*domain.Meeting, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM meetings m
		WHERE m.reminded_at IS NULL AND m.starts_at >= $1 AND m.starts_at < $2 ORDER BY m.starts_at, m.id`, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

// SaveReminder claims the reminder with a conditional update, so concurrent replicas send it once.
func (r *Repository) SaveReminder(ctx context.Context, m *domain.Meeting) error {
	events := m.PullEvents()
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE meetings SET reminded_at = now() WHERE id = $1 AND reminded_at IS NULL`, string(m.ID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrAlreadyReminded
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}
