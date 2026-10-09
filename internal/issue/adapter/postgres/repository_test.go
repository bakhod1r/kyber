package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		pool := dbtest.New(t)
		ctx := context.Background()
		_, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber'),
			('10000000-0000-4000-8000-000000000002','OPS','Ops')`)
		if err != nil {
			t.Fatal(err)
		}
		outbox := func() []string {
			rows, err := pool.Query(ctx, `SELECT name FROM outbox ORDER BY id`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var names []string
			for rows.Next() {
				var n string
				_ = rows.Scan(&n)
				names = append(names, n)
			}
			return names
		}
		return postgres.NewRepository(pool), outbox
	})
}
