package postgres_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/workspace/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
	"github.com/bakhod1r/kyber/internal/workspace/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func(domain.UserID)) {
		pool := dbtest.New(t)
		return postgres.NewRepository(pool), func(u domain.UserID) {
			if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`,
				string(u), string(u)+"@x.uz"); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// The 0014 migration puts every existing project into the default workspace.
func TestDefaultWorkspaceSeeded(t *testing.T) {
	w, err := postgres.NewRepository(dbtest.New(t)).BySlug(context.Background(), domain.DefaultSlug)
	if err != nil || w.ID() != domain.DefaultID {
		t.Fatalf("default = %+v %v", w, err)
	}
}
