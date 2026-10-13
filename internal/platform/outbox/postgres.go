package outbox

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore claims rows with FOR UPDATE SKIP LOCKED, so concurrent replicas
// never process the same event, and marks them published in the same transaction.
type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

const columns = `id, name, payload, created_at, workspace_id::text, actor_id, trace_parent, schema_version, attempts, last_error`

func scanMessage(row pgx.CollectableRow) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.Name, &m.Payload, &m.At, &m.Workspace, &m.Actor, &m.Trace, &m.Version, &m.Attempts, &m.LastError)
	return m, err
}

// Process hands claimed messages to fn in order. A failure is counted on the message and stops
// the batch, except a DeadLetter: that message is parked and the batch goes on.
func (s *PostgresStore) Process(ctx context.Context, limit int, fn func(Message) error) (int, error) {
	var published int
	var handlerErr error
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+columns+` FROM outbox WHERE published_at IS NULL AND dead_at IS NULL
			ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}
		msgs, err := pgx.CollectRows(rows, scanMessage)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(msgs))
		for _, m := range msgs {
			err := fn(m)
			if err == nil {
				ids = append(ids, m.ID)
				continue
			}
			var dl *DeadLetter
			dead := errors.As(err, &dl)
			if _, err := tx.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1, last_error = $2,
				dead_at = CASE WHEN $3 THEN now() END WHERE id = $1`, m.ID, err.Error(), dead); err != nil {
				return err
			}
			if !dead {
				handlerErr = err
				break
			}
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

// DeadLetters lists parked messages of every workspace, oldest first (an operator's view).
func (s *PostgresStore) DeadLetters(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM outbox WHERE dead_at IS NOT NULL ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMessage)
}

// Retry returns a dead letter to the queue with its attempts reset; it is delivered in id order.
func (s *PostgresStore) Retry(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox SET dead_at = NULL, attempts = 0, last_error = '' WHERE id = $1 AND dead_at IS NOT NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotDead
	}
	return nil
}

// DeadLettersOf lists one workspace's parked messages, oldest first (its administrators' view).
func (s *PostgresStore) DeadLettersOf(ctx context.Context, workspace string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+columns+` FROM outbox WHERE dead_at IS NOT NULL AND workspace_id::text = $1
		ORDER BY id LIMIT $2`, workspace, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanMessage)
}

// RetryOf retries a dead letter of the workspace; ErrNotDead for any other message.
func (s *PostgresStore) RetryOf(ctx context.Context, workspace string, id int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE outbox SET dead_at = NULL, attempts = 0, last_error = ''
		WHERE id = $1 AND dead_at IS NOT NULL AND workspace_id::text = $2`, id, workspace)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotDead
	}
	return nil
}
