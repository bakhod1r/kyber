package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/adapter/memory"
	"github.com/bakhod1r/kyber/internal/chat/domain/repotest"
)

func TestContract(t *testing.T) {
	r := memory.NewRepository()
	repotest.Run(t, r, r.Messages())
}
