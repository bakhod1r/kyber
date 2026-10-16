package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/domain/repotest"
	"github.com/bakhod1r/kyber/internal/importer/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.MappingRepository { return postgres.NewRepository(dbtest.New(t)) })
}

func TestLockContract(t *testing.T) {
	repotest.RunLock(t, func(t *testing.T) (domain.ImportLock, domain.ImportLock) {
		pool := dbtest.New(t)
		return postgres.NewImportLock(pool), postgres.NewImportLock(pool)
	})
}

// Holding import locks must not use up the connection pool: the import's own queries need it (a held pool
// connection per lock deadlocked imports on small pools).
func TestLockDoesNotHoldPoolConnections(t *testing.T) {
	cfg := dbtest.New(t).Config()
	cfg.MaxConns = 1
	small, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(small.Close)
	lock := postgres.NewImportLock(small)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, project := range []string{"KYB", "OPS"} {
		release, err := lock.Lock(ctx, project)
		if err != nil {
			t.Fatalf("lock %s: %v", project, err)
		}
		t.Cleanup(release)
	}
	if _, err := small.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("pool starved by held locks: %v", err)
	}
}
