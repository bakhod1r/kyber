package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/webhook/domain/repotest"
	"github.com/bakhod1r/kyber/internal/webhook/infrastructure/memory"
)

func TestContract(t *testing.T) {
	s := memory.NewStore()
	repotest.Run(t, repotest.Repos{Webhooks: s.Webhooks(), Deliveries: s.Deliveries(), Links: s.Links()})
}
