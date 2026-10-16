package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/domain/repotest"
	"github.com/bakhod1r/kyber/internal/importer/infrastructure/memory"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) domain.MappingRepository { return memory.NewRepository() })
}

func TestLockContract(t *testing.T) {
	repotest.RunLock(t, func(*testing.T) (domain.ImportLock, domain.ImportLock) {
		l := memory.NewImportLock()
		return l, l
	})
}
