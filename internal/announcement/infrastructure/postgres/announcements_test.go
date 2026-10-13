package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/announcement/domain/repotest"
	"github.com/bakhod1r/kyber/internal/announcement/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
		t.Fatal(err)
	}
	repotest.Run(t, postgres.NewRepository(pool))
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM outbox WHERE name = 'announcement.published'`).Scan(&n)
	if n != 4 {
		t.Fatalf("outbox events = %d", n)
	}
}
