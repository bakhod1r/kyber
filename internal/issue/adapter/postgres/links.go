package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type LinkRepository struct{ pool *pgxpool.Pool }

func NewLinkRepository(pool *pgxpool.Pool) *LinkRepository { return &LinkRepository{pool: pool} }

func (r *LinkRepository) Add(ctx context.Context, l domain.Link, events []domain.Event) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if l.Type == domain.LinkRelates { // no direction: b relates a equals a relates b
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM issue_links WHERE workspace_id = $1 AND type = 'relates'
				AND from_id = $2 AND to_id = $3`, tenant.Workspace(ctx), string(l.To), string(l.From)).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return domain.ErrLinkExists
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO issue_links (id, workspace_id, type, from_id, to_id) VALUES ($1, $2, $3, $4, $5)`,
			string(l.ID), tenant.Workspace(ctx), string(l.Type), string(l.From), string(l.To))
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrLinkExists
		}
		if err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

func (r *LinkRepository) Remove(ctx context.Context, id domain.LinkID, events []domain.Event) error {
	if !kid.Valid(string(id)) {
		return domain.ErrLinkNotFound
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM issue_links WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrLinkNotFound
		}
		return db.WriteOutbox(ctx, tx, events)
	})
}

const linkCols = `SELECT l.id::text, l.type, l.from_id::text, f.project_key, f.number, l.to_id::text, t.project_key, t.number
	FROM issue_links l JOIN issues f ON f.id = l.from_id JOIN issues t ON t.id = l.to_id`

func scanLink(row pgx.Row) (domain.Link, error) {
	var l domain.Link
	var id, typ, from, fp, to, tp string
	var fn, tn int
	if err := row.Scan(&id, &typ, &from, &fp, &fn, &to, &tp, &tn); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return l, domain.ErrLinkNotFound
		}
		return l, err
	}
	l.ID, l.Type, l.From, l.To = domain.LinkID(id), domain.LinkType(typ), domain.IssueID(from), domain.IssueID(to)
	l.FromKey, _ = domain.NewIssueKey(fp, fn)
	l.ToKey, _ = domain.NewIssueKey(tp, tn)
	return l, nil
}

func (r *LinkRepository) ByID(ctx context.Context, id domain.LinkID) (domain.Link, error) {
	if !kid.Valid(string(id)) {
		return domain.Link{}, domain.ErrLinkNotFound
	}
	return scanLink(r.pool.QueryRow(ctx, linkCols+` WHERE l.id = $1 AND l.workspace_id = $2`, string(id), tenant.Workspace(ctx)))
}

func (r *LinkRepository) ForIssue(ctx context.Context, issue domain.IssueID) ([]domain.Link, error) {
	rows, err := r.pool.Query(ctx, linkCols+` WHERE l.workspace_id = $1 AND (l.from_id = $2 OR l.to_id = $2)
		ORDER BY l.created_at, l.id`, tenant.Workspace(ctx), string(issue))
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Link, error) { return scanLink(row) })
	if out == nil {
		out = []domain.Link{}
	}
	return out, err
}
