package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/audit/domain"
	"github.com/bakhod1r/kyber/internal/audit/domain/repotest"
	"github.com/bakhod1r/kyber/internal/audit/infrastructure/memory"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.Repository { return memory.NewRepository() })
}
