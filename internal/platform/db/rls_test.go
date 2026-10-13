package db_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Row-level security (ADR-0010): a request scoped to one workspace never reads or writes another's rows.
func TestRowLevelSecurity(t *testing.T) {
	pool := dbtest.New(t)
	bg := context.Background()
	const other, user = "00000000-0000-4000-8000-0000000000aa", "00000000-0000-4000-8000-0000000000bb"
	if _, err := pool.Exec(bg, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(bg, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'u@x.uz', 'U', 'x')`, user); err != nil {
		t.Fatal(err)
	}
	for _, ws := range []string{tenant.Default, other} {
		if _, err := pool.Exec(bg, `INSERT INTO saved_filters (id, workspace_id, owner_id, name, jql, version) VALUES (gen_random_uuid(), $1, $2, 'f', 'x', 1)`, ws, user); err != nil {
			t.Fatal(err)
		}
	}
	count := func(ctx context.Context) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM saved_filters`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	scoped := tenant.With(bg, other)
	if n := count(scoped); n != 1 {
		t.Fatalf("scoped sees %d rows, want 1", n)
	}
	if n := count(bg); n != 2 {
		t.Fatalf("unscoped sees %d rows, want 2 (the setting must not stick to the connection)", n)
	}
	if _, err := pool.Exec(scoped, `INSERT INTO saved_filters (id, workspace_id, owner_id, name, jql, version) VALUES (gen_random_uuid(), $1, $2, 'g', 'x', 1)`, tenant.Default, user); err == nil {
		t.Fatal("wrote a row into another workspace")
	}
	tag, err := pool.Exec(scoped, `DELETE FROM saved_filters WHERE workspace_id = $1`, tenant.Default)
	if err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("deleted another workspace's rows: %v %v", tag, err)
	}
	cancelled, cancel := context.WithCancel(scoped)
	cancel()
	pool.Reset() // a fresh connection must be scoped before use; a cancelled request fails instead of running unscoped
	if _, err := pool.Exec(cancelled, `SELECT 1`); err == nil {
		t.Fatal("cancelled request ran")
	}
}
