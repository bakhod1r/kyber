package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/docs/domain/repotest"
	"github.com/bakhod1r/kyber/internal/docs/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
		t.Fatal(err)
	}
	repotest.Run(t, postgres.NewRepository(pool))
}
