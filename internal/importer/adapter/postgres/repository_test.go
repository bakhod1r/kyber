package postgres_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.MappingRepository { return postgres.NewRepository(dbtest.New(t)) })
}
