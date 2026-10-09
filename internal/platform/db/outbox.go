package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Event is any domain event; its JSON form is the published payload.
type Event interface{ EventName() string }

// WriteOutbox appends events inside the caller's transaction (transactional outbox).
func WriteOutbox[E Event](ctx context.Context, tx pgx.Tx, events []E) error {
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", e.EventName(), err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox (name, payload) VALUES ($1, $2)`, e.EventName(), payload); err != nil {
			return err
		}
	}
	return nil
}
