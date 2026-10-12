package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/access/adapter/memory"
	"github.com/bakhod1r/kyber/internal/access/domain/repotest"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }
