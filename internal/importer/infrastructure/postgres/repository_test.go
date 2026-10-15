package postgres_test

import (
	"testing"

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
