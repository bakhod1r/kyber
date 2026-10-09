package outbox

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore claims rows with FOR UPDATE SKIP LOCKED, so concurrent replicas
// never process the same event, and marks them published in the same transaction.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Process(ctx context.Context, limit int, fn func(Message) error) (int, error) {
	var published int
	var handlerErr error
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, name, payload, created_at FROM outbox WHERE published_at IS NULL
			ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}
		msgs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
			var m Message
			err := row.Scan(&m.ID, &m.Name, &m.Payload, &m.At)
			return m, err
		})
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(msgs))
		for _, m := range msgs {
			if handlerErr = fn(m); handlerErr != nil {
				break
			}
			ids = append(ids, m.ID)
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
				return err
			}
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, handlerErr
}
