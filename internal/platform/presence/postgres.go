package presence

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

// Postgres shares the seats of every replica through one table.
type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Beat(ctx context.Context, replica string, seats []Seat, at time.Time) error {
	ws, users := make([]string, 0, len(seats)), make([]string, 0, len(seats))
	for _, s := range seats {
		if kid.Valid(s.Workspace) && kid.Valid(s.User) {
			ws, users = append(ws, s.Workspace), append(users, s.User)
		}
	}
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM presence WHERE replica = $1`, replica); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO presence (replica, workspace_id, user_id, seen_at)
			SELECT DISTINCT $1, w::uuid, u::uuid, $4::timestamptz FROM unnest($2::text[], $3::text[]) AS s(w, u)`,
			replica, ws, users, at)
		return err
	})
}

func (p *Postgres) Online(ctx context.Context, ws string, users []string, since time.Time) ([]string, error) {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if kid.Valid(u) {
			ids = append(ids, u)
		}
	}
	out := []string{}
	if !kid.Valid(ws) || len(ids) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT DISTINCT user_id::text FROM presence
		WHERE workspace_id = $1 AND user_id = ANY($2::uuid[]) AND seen_at > $3`, ws, ids, since)
	if err != nil {
		return nil, err
	}
	on, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, u := range users { // request order, no duplicates
		for _, o := range on {
			if o == u && !contains(out, u) {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

func (p *Postgres) Expire(ctx context.Context, before time.Time) (int, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM presence WHERE seen_at <= $1`, before)
	return int(tag.RowsAffected()), err
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

var _ Store = (*Postgres)(nil)
