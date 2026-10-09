package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.Repository {
		pool := dbtest.New(t)
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES
			($1, 'alice@x.uz', 'Alice', 'h'), ($2, 'bob@x.uz', 'Bob', 'h')`, repotest.Alice, repotest.Bob); err != nil {
			t.Fatal(err)
		}
		return postgres.NewRepository(pool)
	})
}
