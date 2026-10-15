package postgres

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// ImportLock is a PostgreSQL session advisory lock per workspace and project, so replicas sharing the
// database run one import of a project at a time (QA-08). The lock lives on a connection held for the run.
type ImportLock struct{ pool *pgxpool.Pool }

func NewImportLock(pool *pgxpool.Pool) *ImportLock { return &ImportLock{pool: pool} }

func (l *ImportLock) Lock(ctx context.Context, project string) (func(), error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := "kyber:import:" + tenant.Workspace(ctx) + "/" + project
	// pg_advisory_lock waits; cancelling ctx cancels the statement, so the wait ends with the request.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, key); err != nil {
		conn.Release()
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			// Unlock on a fresh context: the request may already be gone. A failed unlock drops the
			// connection, which ends the session and its lock.
			if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key); err != nil {
				_ = conn.Conn().Close(context.Background())
			}
			conn.Release()
		})
	}, nil
}
