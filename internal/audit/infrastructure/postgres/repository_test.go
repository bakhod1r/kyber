package postgres_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/audit/domain/repotest"
	"github.com/bakhod1r/kyber/internal/audit/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.Repository { return postgres.NewRepository(dbtest.New(t)) })
}
