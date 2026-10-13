// Package postgres is the PostgreSQL workspace store.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, w *domain.Workspace) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, $2, $3)`, string(w.ID()), w.Slug(), w.Name())
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrSlugTaken
		}
		if err != nil {
			return err
		}
		return insertMembers(ctx, tx, w)
	})
}

func insertMembers(ctx context.Context, tx pgx.Tx, w *domain.Workspace) error {
	for _, m := range w.Members() {
		if _, err := tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			string(w.ID()), string(m.UserID), string(m.Role)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) load(ctx context.Context, where string, arg any) (*domain.Workspace, error) {
	var wid, slug, name string
	err := r.pool.QueryRow(ctx, `SELECT id::text, slug, name FROM workspaces WHERE `+where, arg).Scan(&wid, &slug, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT user_id::text, role FROM workspace_members WHERE workspace_id = $1`, wid)
	if err != nil {
		return nil, err
	}
	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Member, error) {
		var m domain.Member
		return m, row.Scan(&m.UserID, &m.Role)
	})
	if err != nil {
		return nil, err
	}
	return domain.Rehydrate(domain.WorkspaceID(wid), slug, name, members), nil
}

func (r *Repository) BySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	return r.load(ctx, "slug = $1", slug)
}

func (r *Repository) ByID(ctx context.Context, wid domain.WorkspaceID) (*domain.Workspace, error) {
	if !id.Valid(string(wid)) {
		return nil, domain.ErrWorkspaceNotFound
	}
	return r.load(ctx, "id = $1", string(wid))
}

func (r *Repository) ForUser(ctx context.Context, u domain.UserID) ([]*domain.Workspace, error) {
	rows, err := r.pool.Query(ctx, `SELECT w.slug FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		WHERE m.user_id::text = $1 ORDER BY w.slug`, string(u))
	if err != nil {
		return nil, err
	}
	slugs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Workspace, 0, len(slugs))
	for _, s := range slugs {
		w, err := r.BySlug(ctx, s)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// SaveMembers replaces the member list atomically.
func (r *Repository) SaveMembers(ctx context.Context, w *domain.Workspace) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM workspace_members WHERE workspace_id = $1`, string(w.ID())); err != nil {
			return err
		}
		return insertMembers(ctx, tx, w)
	})
}

func (r *Repository) AddMember(ctx context.Context, w domain.WorkspaceID, u domain.UserID, role domain.Role) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO NOTHING`, string(w), string(u), string(role))
	return err
}
