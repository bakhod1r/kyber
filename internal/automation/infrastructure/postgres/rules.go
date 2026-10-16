// Package postgres stores automation rules and their runs in PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/automation/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, rule *domain.Rule) error {
	actions, err := json.Marshal(rule.Actions)
	if err != nil {
		return err
	}
	args := []any{string(rule.ID), tenant.Workspace(ctx), rule.Project, rule.Owner, rule.Name, string(rule.Trigger), rule.ToStatus,
		rule.Condition, actions, rule.Enabled}
	sql := `INSERT INTO automation_rules (id, workspace_id, project_key, owner_id, name, trigger, to_status, condition, actions, enabled, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1) ON CONFLICT (id) DO NOTHING`
	if rule.Version > 0 {
		sql = `UPDATE automation_rules SET name = $5, trigger = $6, to_status = $7, condition = $8, actions = $9, enabled = $10,
			version = version + 1 WHERE id = $1 AND workspace_id = $2 AND project_key = $3 AND owner_id = $4 AND version = $11`
		args = append(args, rule.Version)
	}
	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrRuleConflict
	}
	rule.Version++
	return nil
}

const cols = `SELECT id::text, project_key, owner_id::text, name, trigger, to_status, condition, actions, enabled, version FROM automation_rules`

func scan(row pgx.CollectableRow) (*domain.Rule, error) {
	var r domain.Rule
	var actions []byte
	if err := row.Scan(&r.ID, &r.Project, &r.Owner, &r.Name, &r.Trigger, &r.ToStatus, &r.Condition, &actions, &r.Enabled, &r.Version); err != nil {
		return nil, err
	}
	return &r, json.Unmarshal(actions, &r.Actions)
}

func (r *Repository) ByID(ctx context.Context, id domain.RuleID) (*domain.Rule, error) {
	if !kid.Valid(string(id)) {
		return nil, domain.ErrRuleNotFound
	}
	rows, err := r.pool.Query(ctx, cols+` WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	if err != nil {
		return nil, err
	}
	rule, err := pgx.CollectExactlyOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrRuleNotFound
	}
	return rule, err
}

func (r *Repository) ForProject(ctx context.Context, project string) ([]*domain.Rule, error) {
	rows, err := r.pool.Query(ctx, cols+` WHERE workspace_id = $1 AND project_key = $2 ORDER BY name, id`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Delete(ctx context.Context, id domain.RuleID) error {
	if !kid.Valid(string(id)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM automation_rules WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	return err
}

func (r *Repository) Record(ctx context.Context, run domain.Run) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO automation_runs (workspace_id, rule_id, project_key, issue_key, at, ok, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, tenant.Workspace(ctx), string(run.Rule), run.Project, run.IssueKey, run.At, run.OK, run.Detail)
	return err
}

func (r *Repository) Runs(ctx context.Context, project string, limit int) ([]domain.Run, error) {
	rows, err := r.pool.Query(ctx, `SELECT rule_id::text, project_key, issue_key, at, ok, detail FROM automation_runs
		WHERE workspace_id = $1 AND project_key = $2 ORDER BY at DESC, id DESC LIMIT $3`, tenant.Workspace(ctx), project, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Run, error) {
		var run domain.Run
		err := row.Scan(&run.Rule, &run.Project, &run.IssueKey, &run.At, &run.OK, &run.Detail)
		run.At = run.At.UTC()
		return run, err
	})
}
