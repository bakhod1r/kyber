package live

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// channel is the PostgreSQL NOTIFY channel shared by all replicas.
const channel = "kyber_live"

type wire struct {
	Workspace string `json:"w"`
	Project   string `json:"p"`
	Event     string `json:"e"`
}

// Postgres broadcasts through NOTIFY, so a change relayed on one replica reaches the streams of
// every replica; Listen feeds this replica's hub.
type Postgres struct {
	pool  *pgxpool.Pool
	hub   *Hub
	log   *slog.Logger
	retry time.Duration
}

func NewPostgres(pool *pgxpool.Pool, hub *Hub, log *slog.Logger) *Postgres {
	return &Postgres{pool: pool, hub: hub, log: log, retry: time.Second}
}

func (p *Postgres) Broadcast(ctx context.Context, c Change) error {
	payload, _ := json.Marshal(wire{Workspace: c.Workspace, Project: c.Project, Event: c.Event})
	_, err := p.pool.Exec(ctx, `SELECT pg_notify($1, $2)`, channel, string(payload))
	return err
}

// Listen relays notifications into the hub until ctx ends, reconnecting after failures.
func (p *Postgres) Listen(ctx context.Context) {
	for ctx.Err() == nil {
		if err := p.listenOnce(ctx); err != nil && ctx.Err() == nil {
			p.log.WarnContext(ctx, "live: listener reconnecting", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(p.retry):
			}
		}
	}
}

func (p *Postgres) listenOnce(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN "+channel)
	for err == nil {
		var n *pgconn.Notification
		if n, err = conn.Conn().WaitForNotification(ctx); err == nil {
			var w wire
			if json.Unmarshal([]byte(n.Payload), &w) == nil {
				p.hub.Publish(Change{Workspace: w.Workspace, Project: w.Project, Event: w.Event})
			}
		}
	}
	return err
}
