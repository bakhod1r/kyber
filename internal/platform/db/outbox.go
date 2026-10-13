package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// Event is any domain event; its JSON form is the published payload.
type Event interface{ EventName() string }

// WriteOutbox appends events with their envelope (workspace, actor, trace context, schema version)
// inside the caller's transaction (transactional outbox).
func WriteOutbox[E Event](ctx context.Context, tx pgx.Tx, events []E) error {
	env := outbox.MetaOf(ctx, time.Time{}) // created_at is the database's clock
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", e.EventName(), err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO outbox (name, payload, workspace_id, actor_id, trace_parent, schema_version)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			e.EventName(), payload, env.Workspace, env.Actor, env.Trace, outbox.SchemaVersion); err != nil {
			return err
		}
	}
	return nil
}
