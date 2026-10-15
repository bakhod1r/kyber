package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/ssosync/domain/repotest"
	"github.com/bakhod1r/kyber/internal/ssosync/infrastructure/postgres"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'sso', 'SSO')`, repotest.WS); err != nil {
		t.Fatal(err)
	}
	for i, u := range []string{repotest.User, repotest.Admin} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`, u, []string{"u@sso.uz", "a@sso.uz"}[i]); err != nil {
			t.Fatal(err)
		}
	}
	s := postgres.New(pool)
	repotest.Run(t, s, s)
}
