// Package dbtest gives each integration test an isolated, migrated schema.
// Tests are skipped unless KYBER_TEST_DATABASE_URL is set.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/db"
)

// Role is the unprivileged role test pools run as.
const Role = "kyber_test_app"

func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("KYBER_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KYBER_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	var b [6]byte
	_, _ = rand.Read(b[:])
	schema := "t_" + hex.EncodeToString(b[:])

	admin, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Tests run as a plain role: superusers bypass row-level security (ADR-0010).
	if _, err := admin.Exec(ctx, `DO $$ BEGIN CREATE ROLE `+Role+` NOSUPERUSER NOBYPASSRLS;
		EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema+" AUTHORIZATION "+Role); err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgxpool.ParseConfig(url)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["role"] = Role
	pool, err := db.OpenConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}
