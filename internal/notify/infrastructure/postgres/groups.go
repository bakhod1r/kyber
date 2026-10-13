package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type GroupRepository struct{ pool *pgxpool.Pool }

func NewGroupRepository(pool *pgxpool.Pool) *GroupRepository { return &GroupRepository{pool: pool} }

func (r *GroupRepository) CreateGroupToken(ctx context.Context, t domain.GroupToken) error {
	_, err := r.pool.Exec(ctx, `WITH gone AS (DELETE FROM telegram_group_tokens WHERE expires_at < now())
		INSERT INTO telegram_group_tokens (hash, workspace_id, project_key, user_id, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		t.Hash, t.Workspace, t.Project, string(t.User), t.ExpiresAt)
	return err
}

func (r *GroupRepository) ConsumeGroupToken(ctx context.Context, hash string, now time.Time) (domain.GroupToken, error) {
	t := domain.GroupToken{Hash: hash}
	var user string
	err := r.pool.QueryRow(ctx, `DELETE FROM telegram_group_tokens WHERE hash = $1 AND expires_at >= $2
		RETURNING workspace_id::text, project_key, user_id::text, expires_at`, hash, now).Scan(&t.Workspace, &t.Project, &user, &t.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupToken{}, domain.ErrGroupTokenInvalid
	}
	t.User = domain.UserID(user)
	return t, err
}

func (r *GroupRepository) LinkChat(ctx context.Context, c domain.ProjectChat) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO project_telegram_chats (workspace_id, project_key, chat_id, title, linked_by, linked_at)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (workspace_id, project_key, chat_id) DO UPDATE SET title = EXCLUDED.title`,
		c.Workspace, c.Project, c.ChatID, c.Title, string(c.LinkedBy), c.LinkedAt)
	return err
}

func (r *GroupRepository) UnlinkChat(ctx context.Context, project, chatID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project_telegram_chats WHERE workspace_id = $1 AND project_key = $2 AND chat_id = $3`,
		tenant.Workspace(ctx), project, chatID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrChatNotLinked
	}
	return nil
}

func (r *GroupRepository) ChatsOf(ctx context.Context, project string) ([]domain.ProjectChat, error) {
	rows, err := r.pool.Query(ctx, `SELECT workspace_id::text, project_key, chat_id, title, COALESCE(linked_by::text, ''), linked_at
		FROM project_telegram_chats WHERE workspace_id = $1 AND project_key = $2 ORDER BY chat_id`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ProjectChat, error) {
		var c domain.ProjectChat
		var by string
		err := row.Scan(&c.Workspace, &c.Project, &c.ChatID, &c.Title, &by, &c.LinkedAt)
		c.LinkedBy, c.LinkedAt = domain.UserID(by), c.LinkedAt.UTC()
		return c, err
	})
	if out == nil {
		out = []domain.ProjectChat{}
	}
	return out, err
}
