package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/agile/adapter/memory"
	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/agile/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		r := memory.NewRepository()
		return r, r.OutboxNames
	})
}
