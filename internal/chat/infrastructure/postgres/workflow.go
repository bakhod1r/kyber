package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Workflows stores workflows (S80).
type Workflows struct{ pool *pgxpool.Pool }

func NewWorkflows(pool *pgxpool.Pool) *Workflows { return &Workflows{pool: pool} }

const workflowCols = `id::text, workspace_id::text, conversation_id::text, trigger, minute, days, text, created_by::text, created_at, last_run`

func scanWorkflow(row pgx.CollectableRow) (*domain.Workflow, error) {
	var w domain.Workflow
	var days int16
	if err := row.Scan(&w.ID, &w.Workspace, &w.Conversation, &w.Trigger, &w.Minute, &days, &w.Text, &w.CreatedBy, &w.CreatedAt, &w.LastRun); err != nil {
		return nil, err
	}
	w.Days = domain.Days(days)
	w.CreatedAt, w.LastRun = w.CreatedAt.UTC(), w.LastRun.UTC()
	return &w, nil
}

func (r *Workflows) Add(ctx context.Context, w *domain.Workflow) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO chat_workflows (id, workspace_id, conversation_id, trigger, minute, days, text, created_by, created_at, last_run)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, string(w.ID), string(w.Workspace), string(w.Conversation), string(w.Trigger),
		w.Minute, int16(w.Days), w.Text, string(w.CreatedBy), w.CreatedAt, w.LastRun)
	return err
}

func (r *Workflows) Remove(ctx context.Context, wid domain.WorkflowID) error {
	if !id.Valid(string(wid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM chat_workflows WHERE id = $1`, string(wid))
	return err
}

func (r *Workflows) ByID(ctx context.Context, wid domain.WorkflowID) (*domain.Workflow, error) {
	if !id.Valid(string(wid)) {
		return nil, domain.ErrWorkflowNotFound
	}
	rows, _ := r.pool.Query(ctx, `SELECT `+workflowCols+` FROM chat_workflows WHERE id = $1`, string(wid))
	w, err := pgx.CollectExactlyOneRow(rows, scanWorkflow)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWorkflowNotFound
	}
	return w, err
}

func (r *Workflows) query(ctx context.Context, where string, args ...any) ([]*domain.Workflow, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+workflowCols+` FROM chat_workflows WHERE `+where+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanWorkflow)
}

func (r *Workflows) In(ctx context.Context, conv domain.ConversationID) ([]*domain.Workflow, error) {
	if !id.Valid(string(conv)) {
		return []*domain.Workflow{}, nil
	}
	return r.query(ctx, `conversation_id = $1`, string(conv))
}

func (r *Workflows) Scheduled(ctx context.Context) ([]*domain.Workflow, error) {
	return r.query(ctx, `trigger = 'schedule'`)
}

func (r *Workflows) MarkRan(ctx context.Context, w *domain.Workflow, prev time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE chat_workflows SET last_run = $2 WHERE id = $1 AND last_run = $3`, string(w.ID), w.LastRun, prev)
	return err == nil && tag.RowsAffected() == 1, err
}
