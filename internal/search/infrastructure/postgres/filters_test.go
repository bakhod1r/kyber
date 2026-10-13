package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/search/domain/repotest"
	"github.com/bakhod1r/kyber/internal/search/infrastructure/postgres"
)

func TestFilterContract(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'ann@x.uz', 'Ann', 'h'), ($2, 'ben@x.uz', 'Ben', 'h')`,
		repotest.Ann, repotest.Ben); err != nil {
		t.Fatal(err)
	}
	repotest.RunFilters(t, postgres.NewFilterRepository(pool))
}
