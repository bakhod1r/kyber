package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/adapter/memory"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) domain.Repository { return memory.NewRepository() })
}
