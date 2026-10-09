package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/insights/adapter/memory"
	"github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/insights/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) domain.Repository { return memory.NewRepository() })
}
