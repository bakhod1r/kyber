// Package postgres is the PostgreSQL Chat store.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const convCols = `c.id, c.workspace_id, c.kind, c.name, c.topic, c.private, coalesce(c.dm_key, ''), c.created_by, c.created_at,
	ARRAY(SELECT m.user_id::text FROM conversation_members m WHERE m.conversation_id = c.id ORDER BY m.user_id)`

func scanConv(row pgx.CollectableRow) (*domain.Conversation, error) {
	var c domain.Conversation
	var members []string
	if err := row.Scan(&c.ID, &c.Workspace, &c.Kind, &c.Name, &c.Topic, &c.Private, &c.Key, &c.CreatedBy, &c.CreatedAt, &members); err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	c.Members = make([]domain.UserID, len(members))
	for i, m := range members {
		c.Members[i] = domain.UserID(m)
	}
	return &c, nil
}

func (r *Repository) Create(ctx context.Context, c *domain.Conversation) error {
	var key *string
	if c.Kind == domain.KindDM {
		key = &c.Key
	}
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO conversations (id, workspace_id, kind, name, topic, private, dm_key, created_by, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, string(c.ID), string(c.Workspace), string(c.Kind), c.Name, c.Topic, c.Private,
			key, string(c.CreatedBy), c.CreatedAt); err != nil {
			return err
		}
		for _, m := range c.Members {
			if _, err := tx.Exec(ctx, `INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3)`,
				string(c.ID), string(m), c.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.ConstraintName == "conversations_channel_name" {
		return domain.ErrNameTaken
	}
	return err
}

func (r *Repository) one(ctx context.Context, where string, args ...any) (*domain.Conversation, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+convCols+` FROM conversations c WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	c, err := pgx.CollectExactlyOneRow(rows, scanConv)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrConversationNotFound
	}
	return c, err
}

func (r *Repository) ByID(ctx context.Context, cid domain.ConversationID) (*domain.Conversation, error) {
	if !id.Valid(string(cid)) {
		return nil, domain.ErrConversationNotFound
	}
	return r.one(ctx, `c.id = $1`, string(cid))
}

func (r *Repository) DMByKey(ctx context.Context, ws domain.WorkspaceID, key string) (*domain.Conversation, error) {
	return r.one(ctx, `c.workspace_id = $1 AND c.kind = 'dm' AND c.dm_key = $2`, string(ws), key)
}

func (r *Repository) AddMember(ctx context.Context, cid domain.ConversationID, u domain.UserID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, string(cid), string(u), at)
	return err
}

func (r *Repository) RemoveMember(ctx context.Context, cid domain.ConversationID, u domain.UserID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`, string(cid), string(u))
	return err
}

func (r *Repository) Visible(ctx context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Conversation, error) {
	if !id.Valid(string(u)) || !id.Valid(string(ws)) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+convCols+` FROM conversations c WHERE c.workspace_id = $1 AND (NOT c.private
		OR EXISTS (SELECT 1 FROM conversation_members m WHERE m.conversation_id = c.id AND m.user_id = $2))
		ORDER BY c.name, c.created_at, c.id`, string(ws), string(u))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanConv)
}

func (r *Repository) MarkRead(ctx context.Context, cid domain.ConversationID, u domain.UserID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversation_members SET last_read_at = $3 WHERE conversation_id = $1 AND user_id = $2`,
		string(cid), string(u), at)
	return err
}

func (r *Repository) Unread(ctx context.Context, u domain.UserID) (map[domain.ConversationID]int, error) {
	out := map[domain.ConversationID]int{}
	if !id.Valid(string(u)) {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT m.conversation_id, count(msg.id) FROM conversation_members m
		JOIN messages msg ON msg.conversation_id = m.conversation_id AND msg.created_at > m.last_read_at
			AND msg.author_id <> m.user_id AND NOT msg.deleted
		WHERE m.user_id = $1 GROUP BY m.conversation_id`, string(u))
	if err != nil {
		return nil, err
	}
	var cid string
	var n int
	_, err = pgx.ForEachRow(rows, []any{&cid, &n}, func() error {
		out[domain.ConversationID(cid)] = n
		return nil
	})
	return out, err
}

type Messages struct{ pool *pgxpool.Pool }

func NewMessages(pool *pgxpool.Pool) *Messages { return &Messages{pool: pool} }

const msgCols = `id, conversation_id, author_id, body, created_at, edited_at, deleted`

func scanMsg(row pgx.CollectableRow) (*domain.Message, error) {
	var m domain.Message
	var edited *time.Time
	if err := row.Scan(&m.ID, &m.Conversation, &m.Author, &m.Body, &m.CreatedAt, &edited, &m.Deleted); err != nil {
		return nil, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	if edited != nil {
		m.EditedAt = edited.UTC()
	}
	return &m, nil
}

func editedAt(m *domain.Message) *time.Time {
	if m.EditedAt.IsZero() {
		return nil
	}
	return &m.EditedAt
}

func (s *Messages) Post(ctx context.Context, m *domain.Message) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO messages (`+msgCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		string(m.ID), string(m.Conversation), string(m.Author), m.Body, m.CreatedAt, editedAt(m), m.Deleted)
	return err
}

func (s *Messages) Update(ctx context.Context, m *domain.Message) error {
	tag, err := s.pool.Exec(ctx, `UPDATE messages SET body = $2, edited_at = $3, deleted = $4 WHERE id = $1`,
		string(m.ID), m.Body, editedAt(m), m.Deleted)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrMessageNotFound
	}
	return err
}

func (s *Messages) ByID(ctx context.Context, mid domain.MessageID) (*domain.Message, error) {
	if !id.Valid(string(mid)) {
		return nil, domain.ErrMessageNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages WHERE id = $1`, string(mid))
	if err != nil {
		return nil, err
	}
	m, err := pgx.CollectExactlyOneRow(rows, scanMsg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMessageNotFound
	}
	return m, err
}

func (s *Messages) History(ctx context.Context, conv domain.ConversationID, before domain.MessageID, limit int) ([]*domain.Message, error) {
	if !id.Valid(string(conv)) {
		return nil, nil
	}
	var cursor *string
	if id.Valid(string(before)) {
		cursor = (*string)(&before)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages WHERE conversation_id = $1
		AND ($2::uuid IS NULL OR (created_at, id) < (SELECT created_at, id FROM messages WHERE id = $2))
		ORDER BY created_at DESC, id DESC LIMIT $3`, string(conv), cursor, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMsg)
}
