package presence_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/presence"
	"github.com/bakhod1r/kyber/internal/platform/presence/presencetest"
)

func TestMemoryContract(t *testing.T) { presencetest.Run(t, presence.NewMemory()) }
