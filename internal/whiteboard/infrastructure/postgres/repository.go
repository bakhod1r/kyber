// Package postgres is the PostgreSQL Whiteboard store.
package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (r *Repository) Create(ctx context.Context, b *domain.Whiteboard) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO whiteboards (id, workspace_id, conversation_id, title, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, string(b.ID), string(b.Workspace), string(b.Conversation), b.Title, string(b.CreatedBy), b.CreatedAt)
	return err
}

const boardCols = `id, workspace_id, conversation_id, title, created_by, created_at`

func scanBoard(row pgx.CollectableRow) (*domain.Whiteboard, error) {
	var b domain.Whiteboard
	err := row.Scan(&b.ID, &b.Workspace, &b.Conversation, &b.Title, &b.CreatedBy, &b.CreatedAt)
	b.CreatedAt = b.CreatedAt.UTC()
	b.Elements = map[domain.ElementID]domain.Element{}
	return &b, err
}

func (r *Repository) ByID(ctx context.Context, wid domain.WhiteboardID) (*domain.Whiteboard, error) {
	if !id.Valid(string(wid)) {
		return nil, domain.ErrWhiteboardMissing
	}
	rows, err := r.pool.Query(ctx, `SELECT `+boardCols+` FROM whiteboards WHERE id = $1`, string(wid))
	if err != nil {
		return nil, err
	}
	b, err := pgx.CollectExactlyOneRow(rows, scanBoard)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWhiteboardMissing
	}
	if err != nil {
		return nil, err
	}
	rows, err = r.pool.Query(ctx, `SELECT element_id, kind, x, y, w, h, points, body, color, stroke, fill, dashed, z, version, updated_by, updated_at
		FROM whiteboard_elements WHERE whiteboard_id = $1`, string(wid))
	if err != nil {
		return nil, err
	}
	var raw []byte
	var e domain.Element
	_, err = pgx.ForEachRow(rows, []any{&e.ID, &e.Kind, &e.X, &e.Y, &e.W, &e.H, &raw, &e.Text, &e.Color, &e.Stroke, &e.Fill, &e.Dashed, &e.Z, &e.Version, &e.UpdatedBy, &e.UpdatedAt}, func() error {
		var pts []point
		if err := json.Unmarshal(raw, &pts); err != nil {
			return err
		}
		el := e
		el.UpdatedAt = el.UpdatedAt.UTC()
		el.Points = nil
		for _, p := range pts {
			el.Points = append(el.Points, domain.Point{X: p.X, Y: p.Y})
		}
		b.Elements[el.ID] = el
		return nil
	})
	return b, err
}

func (r *Repository) InConversation(ctx context.Context, conv domain.ConversationID) ([]*domain.Whiteboard, error) {
	if !id.Valid(string(conv)) {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+boardCols+` FROM whiteboards WHERE conversation_id = $1 ORDER BY created_at DESC, id`, string(conv))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBoard)
}

// Save applies the results in one transaction; every write is conditional on the element's previous
// version, and any miss rolls the whole batch back with ErrStale.
func (r *Repository) Save(ctx context.Context, wid domain.WhiteboardID, applied []domain.OpResult) error {
	if !id.Valid(string(wid)) {
		return domain.ErrWhiteboardMissing
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for _, res := range applied {
			e := res.Element
			pts := make([]point, len(e.Points))
			for i, p := range e.Points {
				pts[i] = point{p.X, p.Y}
			}
			raw, err := json.Marshal(pts)
			if err != nil {
				return err
			}
			var sql string
			args := []any{string(wid), string(e.ID), e.Version - 1}
			switch {
			case e.Deleted:
				sql = `DELETE FROM whiteboard_elements WHERE whiteboard_id = $1 AND element_id = $2 AND version = $3`
			case e.Version == 1:
				sql = `INSERT INTO whiteboard_elements (whiteboard_id, element_id, kind, x, y, w, h, points, body, color, version, updated_by, updated_at, stroke, fill, dashed, z)
					SELECT $1, $2, $4, $5, $6, $7, $8, $9, $10, $11, $3 + 1, $12, $13, $14, $15, $16, $17 ON CONFLICT DO NOTHING`
			default:
				sql = `UPDATE whiteboard_elements SET kind = $4, x = $5, y = $6, w = $7, h = $8, points = $9, body = $10, color = $11,
					version = $3 + 1, updated_by = $12, updated_at = $13, stroke = $14, fill = $15, dashed = $16, z = $17 WHERE whiteboard_id = $1 AND element_id = $2 AND version = $3`
			}
			if !e.Deleted {
				args = append(args, string(e.Kind), e.X, e.Y, e.W, e.H, raw, e.Text, e.Color, string(e.UpdatedBy), e.UpdatedAt, int(e.Stroke), e.Fill, e.Dashed, e.Z)
			}
			tag, err := tx.Exec(ctx, sql, args...)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrStale
			}
		}
		return nil
	})
}
