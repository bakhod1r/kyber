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

func TestDeliveryContract(t *testing.T) {
	repotest.RunDeliveries(t, func(t *testing.T) domain.DeliveryRepository { return postgres.NewDeliveryRepository(dbtest.New(t)) })
}

func TestPrefsContract(t *testing.T) {
	repotest.RunPrefs(t, func(t *testing.T) domain.PrefsRepository {
		pool := dbtest.New(t)
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES
			($1, 'alice@x.uz', 'Alice', 'h'), ($2, 'bob@x.uz', 'Bob', 'h')`, repotest.Alice, repotest.Bob); err != nil {
			t.Fatal(err)
		}
		return postgres.NewPrefsRepository(pool)
	})
}

func TestGroupContract(t *testing.T) {
	repotest.RunGroups(t, func(t *testing.T) domain.GroupRepository {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'alice@x.uz', 'Alice', 'h')`, repotest.Alice); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001'),
			('10000000-0000-4000-8000-000000000002','OPS','Ops','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		return postgres.NewGroupRepository(pool)
	})
}
