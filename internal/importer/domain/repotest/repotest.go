// Package repotest is the contract every MappingRepository adapter must pass.
package repotest

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

func Run(t *testing.T, newRepo func(t *testing.T) domain.MappingRepository) {
	t.Run("add and list per project", func(t *testing.T) {
		ctx := context.Background()
		r := newRepo(t)
		for _, m := range []domain.Mapping{mk("KYB", "PROJ-1", "KYB-1"), mk("KYB", "PROJ-2", "KYB-2"), mk("OPS", "PROJ-1", "OPS-1")} {
			if err := r.Add(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
		got, err := r.ForProject(ctx, "KYB")
		if err != nil || len(got) != 2 || got["PROJ-1"] != "KYB-1" || got["PROJ-2"] != "KYB-2" {
			t.Fatalf("KYB = %v, %v", got, err)
		}
		if none, err := r.ForProject(ctx, "NOPE"); err != nil || len(none) != 0 {
			t.Fatalf("empty = %v, %v", none, err)
		}
	})
	t.Run("duplicate external key", func(t *testing.T) {
		ctx := context.Background()
		r := newRepo(t)
		_ = r.Add(ctx, mk("KYB", "PROJ-1", "KYB-1"))
		if err := r.Add(ctx, mk("KYB", "PROJ-1", "KYB-9")); !errors.Is(err, domain.ErrAlreadyImported) {
			t.Fatalf("err = %v", err)
		}
	})
}

func mk(project, external, issue string) domain.Mapping {
	return domain.Mapping{Project: project, ExternalKey: external, IssueKey: issue}
}
