package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/calendar/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/calendar/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	for _, u := range []string{repotest.Lead, repotest.Dev, repotest.Other} {
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`, u, u+"@x.uz"); err != nil {
			t.Fatal(err)
		}
	}
	repotest.Run(t, postgres.NewRepository(pool))
}
