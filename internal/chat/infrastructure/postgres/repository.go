// Package postgres is the PostgreSQL Chat store.
package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
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
			AND msg.author_id <> m.user_id AND NOT msg.deleted AND msg.thread_id IS NULL
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

const insertCols = `id, conversation_id, author_id, body, created_at, edited_at, deleted, thread_id, kind, ref`

// msgCols reads a message (alias m) with its reply count and reactions ("emoji user,user" rows).
const msgCols = `m.id, m.conversation_id, m.author_id, m.body, m.created_at, m.edited_at, m.deleted, coalesce(m.thread_id::text, ''), m.kind, m.ref, coalesce(m.pinned_by::text, ''), m.pinned_at,
	(SELECT count(*) FROM messages r WHERE r.thread_id = m.id AND NOT r.deleted),
	ARRAY(SELECT x.emoji || ' ' || string_agg(x.user_id::text, ',' ORDER BY x.user_id) FROM message_reactions x
		WHERE x.message_id = m.id GROUP BY x.emoji ORDER BY x.emoji)`

func scanMsg(row pgx.CollectableRow) (*domain.Message, error) {
	var m domain.Message
	var edited *time.Time
	var reactions []string
	var pinnedAt *time.Time
	if err := row.Scan(&m.ID, &m.Conversation, &m.Author, &m.Body, &m.CreatedAt, &edited, &m.Deleted, &m.Thread, &m.Kind, &m.Ref, &m.PinnedBy, &pinnedAt, &m.ReplyCount, &reactions); err != nil {
		return nil, err
	}
	for _, r := range reactions {
		emoji, users, _ := strings.Cut(r, " ")
		re := domain.Reaction{Emoji: emoji}
		for _, u := range strings.Split(users, ",") {
			re.Users = append(re.Users, domain.UserID(u))
		}
		m.Reactions = append(m.Reactions, re)
	}
	m.CreatedAt = m.CreatedAt.UTC()
	if pinnedAt != nil {
		m.PinnedAt = pinnedAt.UTC()
	}
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
	var thread *string
	if m.Thread != "" {
		thread = (*string)(&m.Thread)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO messages (`+insertCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			string(m.ID), string(m.Conversation), string(m.Author), m.Body, m.CreatedAt, editedAt(m), m.Deleted, thread, string(m.Kind), m.Ref); err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.ConstraintName == "messages_ref" {
				return domain.ErrDuplicateRef
			}
			return err
		}
		return db.WriteOutbox(ctx, tx, m.PullEvents())
	})
}

// Update rewrites the message and replaces its reactions.
func (s *Messages) Update(ctx context.Context, m *domain.Message) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var pinnedBy *string
		var pinnedAt *time.Time
		if m.PinnedBy != "" {
			pinnedBy, pinnedAt = (*string)(&m.PinnedBy), &m.PinnedAt
		}
		tag, err := tx.Exec(ctx, `UPDATE messages SET body = $2, edited_at = $3, deleted = $4, pinned_by = $5, pinned_at = $6 WHERE id = $1`,
			string(m.ID), m.Body, editedAt(m), m.Deleted, pinnedBy, pinnedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrMessageNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM message_reactions WHERE message_id = $1`, string(m.ID)); err != nil {
			return err
		}
		for _, r := range m.Reactions {
			for _, u := range r.Users {
				if _, err := tx.Exec(ctx, `INSERT INTO message_reactions (message_id, user_id, emoji) VALUES ($1, $2, $3)`,
					string(m.ID), string(u), r.Emoji); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Messages) ByID(ctx context.Context, mid domain.MessageID) (*domain.Message, error) {
	if !id.Valid(string(mid)) {
		return nil, domain.ErrMessageNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages m WHERE m.id = $1`, string(mid))
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
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages m WHERE m.conversation_id = $1 AND m.thread_id IS NULL
		AND ($2::uuid IS NULL OR (m.created_at, m.id) < (SELECT c.created_at, c.id FROM messages c WHERE c.id = $2))
		ORDER BY m.created_at DESC, m.id DESC LIMIT $3`, string(conv), cursor, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMsg)
}

func (s *Messages) Replies(ctx context.Context, parent domain.MessageID) ([]*domain.Message, error) {
	if !id.Valid(string(parent)) {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages m WHERE m.thread_id = $1 ORDER BY m.created_at, m.id`, string(parent))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMsg)
}

func (s *Messages) Pinned(ctx context.Context, conv domain.ConversationID) ([]*domain.Message, error) {
	if !id.Valid(string(conv)) {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages m WHERE m.conversation_id = $1 AND m.pinned_at IS NOT NULL
		ORDER BY m.pinned_at DESC, m.id`, string(conv))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMsg)
}

// Search matches every word as a case-insensitive substring (strpos, so % and _ are plain characters).
func (s *Messages) Search(ctx context.Context, convs []domain.ConversationID, query string, limit int) ([]*domain.Message, error) {
	words := strings.Fields(strings.ToLower(query))
	ids := make([]string, 0, len(convs))
	for _, c := range convs {
		if id.Valid(string(c)) {
			ids = append(ids, string(c))
		}
	}
	if len(words) == 0 || len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages m WHERE m.conversation_id = ANY($1::uuid[]) AND NOT m.deleted
		AND (SELECT bool_and(strpos(lower(m.body), w) > 0) FROM unnest($2::text[]) w)
		ORDER BY m.created_at DESC, m.id DESC LIMIT $3`, ids, words, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMsg)
}
