package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/docs/domain/repotest"
	"github.com/bakhod1r/kyber/internal/docs/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }

func TestCommentsContract(t *testing.T) {
	pages := memory.NewRepository()
	repotest.RunComments(t, pages, memory.NewComments(pages))
}
