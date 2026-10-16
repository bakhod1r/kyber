// Package postgres is the PostgreSQL Project repository.
package postgres

import (
	"context"
	"errors"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// querier is satisfied by both the pool and a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) Create(ctx context.Context, p *domain.Project) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO projects (id, workspace_id, key, name, issue_seq) VALUES ($1, $2, $3, $4, $5)`,
			string(p.ID()), string(p.Workspace()), p.Key(), p.Name(), p.IssueSeq())
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrKeyTaken
		}
		if err != nil {
			return err
		}
		return writeMembers(ctx, tx, p)
	})
}

// Update locks the project row (SELECT … FOR UPDATE) for the duration of fn.
func (r *Repository) Update(ctx context.Context, key string, fn func(*domain.Project) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		p, err := load(ctx, tx, `WHERE workspace_id = $2 AND key = $1 FOR UPDATE`, key, tenant.Workspace(ctx))
		if err != nil {
			return err
		}
		if err := fn(p); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE projects SET name = $2, issue_seq = $3, archived = $4 WHERE id = $1`,
			string(p.ID()), p.Name(), p.IssueSeq(), p.Archived()); err != nil {
			return err
		}
		return writeMembers(ctx, tx, p)
	})
}

func (r *Repository) ByKey(ctx context.Context, key string) (*domain.Project, error) {
	return load(ctx, r.pool, `WHERE workspace_id = $2 AND key = $1`, key, tenant.Workspace(ctx))
}

func (r *Repository) ListForUser(ctx context.Context, u domain.UserID) ([]*domain.Project, error) {
	return r.list(ctx, `SELECT p.key FROM projects p
		JOIN project_members m ON m.project_id = p.id WHERE m.user_id = $1 AND p.workspace_id = $2 ORDER BY p.key`, string(u), tenant.Workspace(ctx))
}

func (r *Repository) InWorkspace(ctx context.Context) ([]*domain.Project, error) {
	return r.list(ctx, `SELECT key FROM projects WHERE workspace_id = $1 ORDER BY key`, tenant.Workspace(ctx))
}

// list loads the projects whose keys the query selects.
func (r *Repository) list(ctx context.Context, query string, args ...any) ([]*domain.Project, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Project, 0, len(keys))
	for _, k := range keys {
		p, err := r.ByKey(ctx, k)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func load(ctx context.Context, q querier, where string, args ...any) (*domain.Project, error) {
	var id, ws, key, name string
	var seq int
	var archived bool
	err := q.QueryRow(ctx, `SELECT id::text, workspace_id::text, key, name, issue_seq, archived FROM projects `+where, args...).
		Scan(&id, &ws, &key, &name, &seq, &archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT user_id::text, role FROM project_members WHERE project_id = $1`, id)
	if err != nil {
		return nil, err
	}
	members, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Member, error) {
		var m domain.Member
		err := row.Scan(&m.UserID, &m.Role)
		return m, err
	})
	if err != nil {
		return nil, err
	}
	return domain.RehydrateArchived(domain.ProjectID(id), domain.WorkspaceID(ws), key, name, seq, members, archived), nil
}

// writeMembers replaces the stored membership with the aggregate's.
func writeMembers(ctx context.Context, tx pgx.Tx, p *domain.Project) error {
	if _, err := tx.Exec(ctx, `DELETE FROM project_members WHERE project_id = $1`, string(p.ID())); err != nil {
		return err
	}
	for _, m := range p.Members() {
		if _, err := tx.Exec(ctx, `INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`,
			string(p.ID()), string(m.UserID), string(m.Role)); err != nil {
			return err
		}
	}
	return nil
}
