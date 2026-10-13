package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/domain/repotest"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/memory"
)

func TestContract(t *testing.T) {
	r := memory.NewRepository()
	repotest.Run(t, r, r.Messages())
}
