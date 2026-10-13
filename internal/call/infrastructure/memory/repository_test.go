package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/call/domain/repotest"
	"github.com/bakhod1r/kyber/internal/call/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }
