package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobtest"
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/memory"
)

func TestBlobs(t *testing.T) { blobtest.Run(t, memory.NewBlobs()) }
