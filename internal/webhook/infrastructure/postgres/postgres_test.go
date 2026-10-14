package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/webhook/domain/repotest"
	"github.com/bakhod1r/kyber/internal/webhook/infrastructure/postgres"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
		t.Fatal(err)
	}
	repotest.Run(t, repotest.Repos{Webhooks: postgres.NewWebhooks(pool), Deliveries: postgres.NewDeliveries(pool), Links: postgres.NewLinks(pool)})
}
