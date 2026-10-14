// Package postgres is the PostgreSQL Call store.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func ended(c *domain.Call) *time.Time {
	if c.Live() {
		return nil
	}
	return &c.EndedAt
}

func writeParticipants(ctx context.Context, tx pgx.Tx, c *domain.Call) error {
	if _, err := tx.Exec(ctx, `DELETE FROM call_participants WHERE call_id = $1`, string(c.ID)); err != nil {
		return err
	}
	for _, p := range c.Participants {
		if _, err := tx.Exec(ctx, `INSERT INTO call_participants (call_id, user_id, joined_at, audio, video, screen, hand) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			string(c.ID), string(p.User), p.JoinedAt, p.Media.Audio, p.Media.Video, p.Media.Screen, p.Hand); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, c *domain.Call) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO calls (id, workspace_id, conversation_id, started_by, started_at, ended_at, version)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, string(c.ID), string(c.Workspace), string(c.Conversation), string(c.StartedBy),
			c.StartedAt, ended(c), c.Version); err != nil {
			return err
		}
		if err := writeParticipants(ctx, tx, c); err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, c.PullEvents())
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.ConstraintName == "calls_one_live" {
		return domain.ErrCallLive
	}
	return err
}

func (r *Repository) Save(ctx context.Context, c *domain.Call) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		kind, ref, by := presentation(c)
		var recBy *string
		var recSince *time.Time
		if r := c.Recording; r != nil {
			recBy, recSince = (*string)(&r.By), &r.Since
		}
		tag, err := tx.Exec(ctx, `UPDATE calls SET ended_at = $3, presentation_kind = $4, presentation_ref = $5, presentation_by = $6,
			recording_by = $7, recording_since = $8, version = version + 1 WHERE id = $1 AND version = $2`,
			string(c.ID), c.Version, ended(c), kind, ref, by, recBy, recSince)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		if err := writeParticipants(ctx, tx, c); err != nil {
			return err
		}
		c.Version++
		return db.WriteOutbox(ctx, tx, c.PullEvents())
	})
}

const cols = `c.id, c.workspace_id, c.conversation_id, c.started_by, c.started_at, c.ended_at, c.version,
	c.presentation_kind, c.presentation_ref, c.presentation_by::text, c.recording_by::text, c.recording_since`

func presentation(c *domain.Call) (kind, ref, by *string) {
	if p := c.Presentation; p != nil {
		k, r, b := string(p.Kind), p.Ref, string(p.By)
		return &k, &r, &b
	}
	return nil, nil, nil
}

func (r *Repository) one(ctx context.Context, where string, arg string) (*domain.Call, error) {
	var c domain.Call
	var end *time.Time
	var pKind, pRef, pBy, recBy *string
	var recSince *time.Time
	err := r.pool.QueryRow(ctx, `SELECT `+cols+` FROM calls c WHERE `+where, arg).
		Scan(&c.ID, &c.Workspace, &c.Conversation, &c.StartedBy, &c.StartedAt, &end, &c.Version, &pKind, &pRef, &pBy, &recBy, &recSince)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrCallNotFound
	}
	if err != nil {
		return nil, err
	}
	c.StartedAt = c.StartedAt.UTC()
	if end != nil {
		c.EndedAt = end.UTC()
	}
	if recBy != nil && recSince != nil {
		c.Recording = &domain.Recording{By: domain.UserID(*recBy), Since: recSince.UTC()}
	}
	if pKind != nil && pRef != nil && pBy != nil {
		c.Presentation = &domain.Presentation{Kind: domain.PresentKind(*pKind), Ref: *pRef, By: domain.UserID(*pBy)}
	}
	rows, err := r.pool.Query(ctx, `SELECT user_id, joined_at, audio, video, screen, hand FROM call_participants WHERE call_id = $1
		ORDER BY joined_at, user_id`, string(c.ID))
	if err != nil {
		return nil, err
	}
	c.Participants, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Participant, error) {
		var p domain.Participant
		err := row.Scan(&p.User, &p.JoinedAt, &p.Media.Audio, &p.Media.Video, &p.Media.Screen, &p.Hand)
		p.JoinedAt = p.JoinedAt.UTC()
		return p, err
	})
	return &c, err
}

func (r *Repository) ByID(ctx context.Context, cid domain.CallID) (*domain.Call, error) {
	if !id.Valid(string(cid)) {
		return nil, domain.ErrCallNotFound
	}
	return r.one(ctx, `c.id = $1`, string(cid))
}

func (r *Repository) LiveIn(ctx context.Context, conv domain.ConversationID) (*domain.Call, error) {
	if !id.Valid(string(conv)) {
		return nil, domain.ErrCallNotFound
	}
	return r.one(ctx, `c.conversation_id = $1 AND c.ended_at IS NULL`, string(conv))
}

func (r *Repository) CountLive(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM calls WHERE ended_at IS NULL`).Scan(&n)
	return n, err
}
