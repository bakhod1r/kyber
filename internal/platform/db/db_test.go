package db_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := dbtest.New(t) // already migrated once
	ctx := context.Background()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 14 {
		t.Fatalf("applied = %d, %v", n, err)
	}
}
