package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/workspace/adapter/memory"
	"github.com/bakhod1r/kyber/internal/workspace/domain"
	"github.com/bakhod1r/kyber/internal/workspace/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) (domain.Repository, func(domain.UserID)) {
		return memory.NewRepository(), func(domain.UserID) {}
	})
}
