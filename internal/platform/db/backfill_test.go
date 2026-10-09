package db_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

// Regression (QA-03-1): projects created before membership existed must stay reachable.
func TestBackfillOrphanedProjects(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, name, password_hash) VALUES
			('40000000-0000-4000-8000-000000000001', 'a@x.uz', 'A', 'h'),
			('40000000-0000-4000-8000-000000000002', 'b@x.uz', 'B', 'h');
		INSERT INTO projects (id, key, name) VALUES
			('50000000-0000-4000-8000-000000000001', 'OLD', 'Legacy'),
			('50000000-0000-4000-8000-000000000002', 'NEW', 'Owned');
		INSERT INTO project_members VALUES
			('50000000-0000-4000-8000-000000000002', '40000000-0000-4000-8000-000000000002', 'admin');`)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := db.MigrationSQL("0003_backfill_project_members.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // idempotent
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	var oldAdmins, newMembers int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM project_members m JOIN projects p ON p.id = m.project_id
		WHERE p.key = 'OLD' AND m.role = 'admin'`).Scan(&oldAdmins)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM project_members m JOIN projects p ON p.id = m.project_id
		WHERE p.key = 'NEW'`).Scan(&newMembers)
	if oldAdmins != 2 || newMembers != 1 {
		t.Fatalf("OLD admins = %d (want 2), NEW members = %d (want 1, untouched)", oldAdmins, newMembers)
	}
}
