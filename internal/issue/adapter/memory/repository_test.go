package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		r := memory.NewRepository()
		return r, r.OutboxNames
	})
}
