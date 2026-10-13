package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/form/domain/repotest"
	"github.com/bakhod1r/kyber/internal/form/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }
