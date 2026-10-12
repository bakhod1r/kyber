package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/access/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.Other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES
		('10000000-0000-4000-8000-000000000001', 'KYB', 'Kyber', '00000000-0000-4000-8000-000000000001'),
		('10000000-0000-4000-8000-000000000002', 'KYB', 'Kyber', $1)`, repotest.Other); err != nil {
		t.Fatal(err)
	}
	repotest.Run(t, postgres.NewRepository(pool))
}
