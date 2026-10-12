package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type WorkflowRepository struct{ pool *pgxpool.Pool }

func NewWorkflowRepository(pool *pgxpool.Pool) *WorkflowRepository {
	return &WorkflowRepository{pool: pool}
}

func (r *WorkflowRepository) ByProject(ctx context.Context, project string) (domain.WorkflowDef, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT definition FROM project_workflows WHERE workspace_id = $1 AND project_key = $2`,
		tenant.Workspace(ctx), project).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowDef{}, domain.ErrWorkflowNotFound
	}
	if err != nil {
		return domain.WorkflowDef{}, err
	}
	var def domain.WorkflowDef
	return def, json.Unmarshal(raw, &def)
}

func (r *WorkflowRepository) Save(ctx context.Context, project string, def domain.WorkflowDef) error {
	raw, err := json.Marshal(def)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO project_workflows (workspace_id, project_key, definition) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, project_key) DO UPDATE SET definition = EXCLUDED.definition, updated_at = now()`,
		tenant.Workspace(ctx), project, raw)
	return err
}
