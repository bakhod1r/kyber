// Package postgres stores forms in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/form/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// question is the stored JSON shape of a question.
type question struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Long     bool   `json:"long"`
}

func toJSON(qs []domain.Question) []question {
	out := make([]question, len(qs))
	for i, q := range qs {
		out[i] = question(q)
	}
	return out
}

func (r *Repository) Save(ctx context.Context, f *domain.Form) error {
	args := []any{string(f.ID), tenant.Workspace(ctx), f.Project, f.Name, f.Intro, f.IssueType, toJSON(f.Questions)}
	sql := `INSERT INTO forms (id, workspace_id, project_key, name, intro, issue_type, questions, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 1) ON CONFLICT (id) DO NOTHING`
	if f.Version > 0 {
		sql = `UPDATE forms SET name = $4, intro = $5, issue_type = $6, questions = $7, version = version + 1
			WHERE id = $1 AND workspace_id = $2 AND project_key = $3 AND version = $8`
		args = append(args, f.Version)
	}
	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrFormConflict
	}
	f.Version++
	return nil
}

const cols = `SELECT id::text, project_key, name, intro, issue_type, questions, version FROM forms`

func scan(row pgx.CollectableRow) (*domain.Form, error) {
	var f domain.Form
	var qs []question
	if err := row.Scan(&f.ID, &f.Project, &f.Name, &f.Intro, &f.IssueType, &qs, &f.Version); err != nil {
		return nil, err
	}
	f.Questions = make([]domain.Question, len(qs))
	for i, q := range qs {
		f.Questions[i] = domain.Question(q)
	}
	return &f, nil
}

func (r *Repository) ByID(ctx context.Context, id domain.FormID) (*domain.Form, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrFormNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	f, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrFormNotFound
	}
	return f, err
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Form, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY name COLLATE "C", id`,
		tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Delete(ctx context.Context, id domain.FormID) error {
	if !kid.Valid(string(id)) {
		return domain.ErrFormNotFound
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM forms WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFormNotFound
	}
	return nil
}
