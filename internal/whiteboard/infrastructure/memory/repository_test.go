package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/whiteboard/domain/repotest"
	"github.com/bakhod1r/kyber/internal/whiteboard/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }

func TestCommentsContract(t *testing.T) {
	repotest.RunComments(t, memory.NewRepository(), memory.NewComments())
}
