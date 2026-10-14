package postgres_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain/repotest"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/postgres"
)

func TestContract(t *testing.T) { repotest.Run(t, postgres.NewRepository(dbtest.New(t))) }

func TestCommentsContract(t *testing.T) {
	pool := dbtest.New(t)
	repotest.RunComments(t, postgres.NewRepository(pool), postgres.NewComments(pool))
}
