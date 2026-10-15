package postgres

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// ImportLock is a PostgreSQL session advisory lock per workspace and project, so replicas sharing the
// database run one import of a project at a time (QA-08). The lock lives on a connection of its own, opened
// for the run outside the pool: a pooled connection held for a whole import starves the import's own
// queries (and the outbox relay) on small pools.
type ImportLock struct{ pool *pgxpool.Pool }

func NewImportLock(pool *pgxpool.Pool) *ImportLock { return &ImportLock{pool: pool} }

func (l *ImportLock) Lock(ctx context.Context, project string) (func(), error) {
	conn, err := pgx.ConnectConfig(ctx, l.pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, err
	}
	key := "kyber:import:" + tenant.Workspace(ctx) + "/" + project
	// pg_advisory_lock waits; cancelling ctx cancels the statement, so the wait ends with the request.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, key); err != nil {
		_ = conn.Close(context.Background())
		return nil, err
	}
	var once sync.Once
	return func() {
		// Closing the session releases its advisory lock, even when the request is already gone.
		once.Do(func() { _ = conn.Close(context.Background()) })
	}, nil
}
