package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Groups stores user groups (S70).
type Groups struct{ pool *pgxpool.Pool }

func NewGroups(pool *pgxpool.Pool) *Groups { return &Groups{pool: pool} }

const groupCols = `id::text, workspace_id::text, handle, name, ARRAY(SELECT m::text FROM unnest(member_ids) m ORDER BY 1), created_by::text, created_at`

func scanGroup(row pgx.CollectableRow) (*domain.Group, error) {
	var g domain.Group
	var members []string
	if err := row.Scan(&g.ID, &g.Workspace, &g.Handle, &g.Name, &members, &g.CreatedBy, &g.CreatedAt); err != nil {
		return nil, err
	}
	for _, m := range members {
		g.Members = append(g.Members, domain.UserID(m))
	}
	g.CreatedAt = g.CreatedAt.UTC()
	return &g, nil
}

func (r *Groups) Save(ctx context.Context, g *domain.Group) error {
	members := make([]string, len(g.Members))
	for i, m := range g.Members {
		members[i] = string(m)
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO user_groups (id, workspace_id, handle, name, member_ids, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5::uuid[], $6, $7)
		ON CONFLICT (id) DO UPDATE SET handle = $3, name = $4, member_ids = $5::uuid[]`,
		string(g.ID), string(g.Workspace), g.Handle, g.Name, members, string(g.CreatedBy), g.CreatedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.ConstraintName == "user_groups_handle" {
		return domain.ErrHandleTaken
	}
	return err
}

func (r *Groups) Delete(ctx context.Context, gid domain.GroupID) error {
	if !id.Valid(string(gid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM user_groups WHERE id = $1`, string(gid))
	return err
}

func (r *Groups) one(ctx context.Context, where string, args ...any) (*domain.Group, error) {
	rows, _ := r.pool.Query(ctx, `SELECT `+groupCols+` FROM user_groups WHERE `+where, args...)
	g, err := pgx.CollectExactlyOneRow(rows, scanGroup)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrGroupNotFound
	}
	return g, err
}

func (r *Groups) ByID(ctx context.Context, gid domain.GroupID) (*domain.Group, error) {
	if !id.Valid(string(gid)) {
		return nil, domain.ErrGroupNotFound
	}
	return r.one(ctx, `id = $1`, string(gid))
}

func (r *Groups) ByHandle(ctx context.Context, ws domain.WorkspaceID, handle string) (*domain.Group, error) {
	if !id.Valid(string(ws)) {
		return nil, domain.ErrGroupNotFound
	}
	return r.one(ctx, `workspace_id = $1 AND handle = $2`, string(ws), handle)
}

func (r *Groups) In(ctx context.Context, ws domain.WorkspaceID) ([]*domain.Group, error) {
	if !id.Valid(string(ws)) {
		return []*domain.Group{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+groupCols+` FROM user_groups WHERE workspace_id = $1 ORDER BY handle`, string(ws))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanGroup)
}
