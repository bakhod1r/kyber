package postgres_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/call/domain/repotest"
	"github.com/bakhod1r/kyber/internal/call/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) { repotest.Run(t, postgres.NewRepository(dbtest.New(t))) }
