package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/ssosync/domain/repotest"
	"github.com/bakhod1r/kyber/internal/ssosync/infrastructure/memory"
)

func TestContract(t *testing.T) {
	s := memory.New()
	repotest.Run(t, s, s)
}
