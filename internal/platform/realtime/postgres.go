package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// channel is the PostgreSQL NOTIFY channel shared by all replicas.
const channel = "kyber_realtime"

// notifyLimit stays under NOTIFY's 8000-byte payload limit; larger events go through realtime_spill.
const notifyLimit = 7000

type wire struct {
	Workspace string          `json:"w"`
	Users     []string        `json:"u"`
	Type      string          `json:"t"`
	Data      json.RawMessage `json:"d,omitempty"`
	Spill     int64           `json:"s,omitempty"` // the realtime_spill row holding the event
}

// Postgres fans events out to every replica through NOTIFY; each replica's Listen feeds its own Hub,
// so a message posted on one node reaches the streams held by another. Event ids are per replica: a
// tab that reconnects to another node reloads instead of replaying (see the web client's onConnect).
type Postgres struct {
	pool  *pgxpool.Pool
	hub   *Hub
	log   *slog.Logger
	retry time.Duration
}

func NewPostgres(pool *pgxpool.Pool, hub *Hub, log *slog.Logger) *Postgres {
	return &Postgres{pool: pool, hub: hub, log: log, retry: time.Second}
}

// Publish has the Hub's signature, so contexts publish through it unchanged.
func (p *Postgres) Publish(ws string, users []string, typ string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ctx := context.Background()
	payload, _ := json.Marshal(wire{Workspace: ws, Users: users, Type: typ, Data: raw})
	if len(payload) > notifyLimit {
		var id int64
		if err := p.pool.QueryRow(ctx, `INSERT INTO realtime_spill (payload) VALUES ($1) RETURNING id`, string(payload)).Scan(&id); err != nil {
			return err
		}
		// Spilled events are read within moments; old ones are swept as new ones arrive.
		_, _ = p.pool.Exec(ctx, `DELETE FROM realtime_spill WHERE created_at < now() - interval '5 minutes'`)
		payload, _ = json.Marshal(wire{Spill: id})
	}
	_, err = p.pool.Exec(ctx, `SELECT pg_notify($1, $2)`, channel, string(payload))
	return err
}

// Listen relays notifications into the hub until ctx ends, reconnecting after failures.
func (p *Postgres) Listen(ctx context.Context) {
	for ctx.Err() == nil {
		if err := p.listenOnce(ctx); err != nil && ctx.Err() == nil {
			p.log.WarnContext(ctx, "realtime: listener reconnecting", "err", err)
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
			p.deliver(ctx, n.Payload)
		}
	}
	return err
}

func (p *Postgres) deliver(ctx context.Context, payload string) {
	var w wire
	if json.Unmarshal([]byte(payload), &w) != nil {
		return
	}
	if w.Spill != 0 {
		var full string
		if err := p.pool.QueryRow(ctx, `SELECT payload FROM realtime_spill WHERE id = $1`, w.Spill).Scan(&full); err != nil ||
			json.Unmarshal([]byte(full), &w) != nil {
			p.log.WarnContext(ctx, "realtime: lost a spilled event", "id", w.Spill, "err", err)
			return
		}
	}
	_ = p.hub.Publish(w.Workspace, w.Users, w.Type, w.Data)
}
