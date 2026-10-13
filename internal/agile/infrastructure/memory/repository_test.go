package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/agile/domain/repotest"
	"github.com/bakhod1r/kyber/internal/agile/infrastructure/memory"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		r := memory.NewRepository()
		return r, r.OutboxNames
	})
}

func TestBoardContract(t *testing.T) {
	repotest.RunBoards(t, func(t *testing.T) domain.BoardRepository { return memory.NewBoardRepository() })
}

func TestVersionContract(t *testing.T) {
	repotest.RunVersions(t, func(t *testing.T) (domain.VersionRepository, func() []string) {
		r := memory.NewVersionRepository()
		return r, r.OutboxNames
	})
}
