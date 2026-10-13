package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/chat/domain"
	"github.com/bakhod1r/kyber/internal/platform/id"
)

// Bookmarks stores bookmarks (S79).
type Bookmarks struct{ pool *pgxpool.Pool }

func NewBookmarks(pool *pgxpool.Pool) *Bookmarks { return &Bookmarks{pool: pool} }

const bookmarkCols = `id::text, workspace_id::text, conversation_id::text, kind, ref, title, added_by::text, added_at`

func scanBookmark(row pgx.CollectableRow) (*domain.Bookmark, error) {
	var b domain.Bookmark
	if err := row.Scan(&b.ID, &b.Workspace, &b.Conversation, &b.Kind, &b.Ref, &b.Title, &b.AddedBy, &b.AddedAt); err != nil {
		return nil, err
	}
	b.AddedAt = b.AddedAt.UTC()
	return &b, nil
}

func (r *Bookmarks) Add(ctx context.Context, b *domain.Bookmark) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO conversation_bookmarks (id, workspace_id, conversation_id, kind, ref, title, added_by, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, string(b.ID), string(b.Workspace), string(b.Conversation), string(b.Kind), b.Ref, b.Title,
		string(b.AddedBy), b.AddedAt)
	return err
}

func (r *Bookmarks) Remove(ctx context.Context, bid domain.BookmarkID) error {
	if !id.Valid(string(bid)) {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM conversation_bookmarks WHERE id = $1`, string(bid))
	return err
}

func (r *Bookmarks) ByID(ctx context.Context, bid domain.BookmarkID) (*domain.Bookmark, error) {
	if !id.Valid(string(bid)) {
		return nil, domain.ErrBookmarkNotFound
	}
	rows, _ := r.pool.Query(ctx, `SELECT `+bookmarkCols+` FROM conversation_bookmarks WHERE id = $1`, string(bid))
	b, err := pgx.CollectExactlyOneRow(rows, scanBookmark)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrBookmarkNotFound
	}
	return b, err
}

func (r *Bookmarks) In(ctx context.Context, conv domain.ConversationID) ([]*domain.Bookmark, error) {
	if !id.Valid(string(conv)) {
		return []*domain.Bookmark{}, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+bookmarkCols+` FROM conversation_bookmarks WHERE conversation_id = $1 ORDER BY added_at, id`, string(conv))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBookmark)
}
