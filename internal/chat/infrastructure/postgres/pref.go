package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Prefs stores conversation settings (S71).
type Prefs struct{ pool *pgxpool.Pool }

func NewPrefs(pool *pgxpool.Pool) *Prefs { return &Prefs{pool: pool} }

func (r *Prefs) Put(ctx context.Context, p *domain.Pref) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO conversation_prefs (workspace_id, user_id, conversation_id, level, section) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, conversation_id) DO UPDATE SET level = $4, section = $5`,
		string(p.Workspace), string(p.User), string(p.Conversation), string(p.Level), p.Section)
	return err
}

func (r *Prefs) query(ctx context.Context, where string, args ...any) ([]*domain.Pref, error) {
	rows, err := r.pool.Query(ctx, `SELECT workspace_id::text, user_id::text, conversation_id::text, level, section FROM conversation_prefs WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Pref, error) {
		var p domain.Pref
		return &p, row.Scan(&p.Workspace, &p.User, &p.Conversation, &p.Level, &p.Section)
	})
}

func (r *Prefs) Mine(ctx context.Context, ws domain.WorkspaceID, u domain.UserID) ([]*domain.Pref, error) {
	if !id.Valid(string(ws)) || !id.Valid(string(u)) {
		return []*domain.Pref{}, nil
	}
	return r.query(ctx, `workspace_id = $1 AND user_id = $2`, string(ws), string(u))
}

func (r *Prefs) Of(ctx context.Context, c domain.ConversationID) ([]*domain.Pref, error) {
	if !id.Valid(string(c)) {
		return []*domain.Pref{}, nil
	}
	return r.query(ctx, `conversation_id = $1`, string(c))
}

// SectionOrders stores section orders (S85).
type SectionOrders struct{ pool *pgxpool.Pool }

func NewSectionOrders(pool *pgxpool.Pool) *SectionOrders { return &SectionOrders{pool: pool} }

func (r *SectionOrders) PutOrder(ctx context.Context, ws domain.WorkspaceID, u domain.UserID, order []string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO chat_section_orders (workspace_id, user_id, sections) VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET sections = $3`, string(ws), string(u), order)
	return err
}

func (r *SectionOrders) Order(ctx context.Context, ws domain.WorkspaceID, u domain.UserID) ([]string, error) {
	if !id.Valid(string(ws)) || !id.Valid(string(u)) {
		return []string{}, nil
	}
	var out []string
	err := r.pool.QueryRow(ctx, `SELECT sections FROM chat_section_orders WHERE workspace_id = $1 AND user_id = $2`, string(ws), string(u)).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	return out, err
}
