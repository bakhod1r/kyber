// Package events provides domain event publishers.
package events

import (
	"context"
	"log/slog"
)

type named interface{ EventName() string }

// LogPublisher logs events; placeholder until the Postgres outbox lands.
type LogPublisher[E named] struct{ Log *slog.Logger }

func (p LogPublisher[E]) Publish(ctx context.Context, events ...E) error {
	for _, e := range events {
		p.Log.InfoContext(ctx, "domain event", "event", e.EventName(), "payload", e)
	}
	return nil
}
