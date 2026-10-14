package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/domain/repotest"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	for _, u := range []string{repotest.Lead, repotest.Dev, repotest.Other} {
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`, u, u+"@x.uz"); err != nil {
			t.Fatal(err)
		}
	}
	repotest.Run(t, postgres.NewRepository(pool), postgres.NewMessages(pool))
}

func TestSavedContract(t *testing.T) {
	pool := dbtest.New(t)
	for _, u := range []string{repotest.Lead, repotest.Dev, repotest.Other} {
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`, u, u+"@x.uz"); err != nil {
			t.Fatal(err)
		}
	}
	repotest.RunSaved(t, postgres.NewRepository(pool), postgres.NewMessages(pool), postgres.NewSaved(pool))
}

func TestStatusContract(t *testing.T) { repotest.RunStatuses(t, postgres.NewStatuses(dbtest.New(t))) }

func TestScheduledContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunScheduled(t, postgres.NewRepository(pool), postgres.NewScheduled(pool), postgres.NewReminders(pool))
}

func TestReceiptsContract(t *testing.T) {
	repotest.RunReceipts(t, postgres.NewRepository(dbtest.New(t)))
}

func TestManagementContract(t *testing.T) {
	repotest.RunManagement(t, postgres.NewRepository(dbtest.New(t)))
}

func TestGroupsContract(t *testing.T) { repotest.RunGroups(t, postgres.NewGroups(dbtest.New(t))) }

func TestPrefsContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunPrefs(t, postgres.NewRepository(pool), postgres.NewPrefs(pool))
}

func TestBookmarksContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunBookmarks(t, postgres.NewRepository(pool), postgres.NewBookmarks(pool))
}

func TestWorkflowsContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunWorkflows(t, postgres.NewRepository(pool), postgres.NewWorkflows(pool))
}

func TestWebhooksContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunWebhooks(t, postgres.NewRepository(pool), postgres.NewWebhooks(pool))
}

func TestBotsContract(t *testing.T) { repotest.RunBots(t, postgres.NewBots(dbtest.New(t))) }

func TestSectionOrdersContract(t *testing.T) {
	repotest.RunSectionOrders(t, postgres.NewSectionOrders(dbtest.New(t)))
}
