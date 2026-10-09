package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/importer/adapter/memory"
	"github.com/bakhod1r/kyber/internal/importer/domain"
	"github.com/bakhod1r/kyber/internal/importer/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) domain.MappingRepository { return memory.NewRepository() })
}
