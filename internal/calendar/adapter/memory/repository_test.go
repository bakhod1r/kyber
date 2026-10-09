package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/calendar/adapter/memory"
	"github.com/bakhod1r/kyber/internal/calendar/domain/repotest"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }
