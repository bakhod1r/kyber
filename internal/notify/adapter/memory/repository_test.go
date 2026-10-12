package memory_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/notify/adapter/memory"
	"github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/notify/domain/repotest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(*testing.T) domain.Repository { return memory.NewRepository() })
}

func TestDeliveryContract(t *testing.T) {
	repotest.RunDeliveries(t, func(t *testing.T) domain.DeliveryRepository { return memory.NewDeliveryRepository() })
}

func TestPrefsContract(t *testing.T) {
	repotest.RunPrefs(t, func(t *testing.T) domain.PrefsRepository { return memory.NewPrefsRepository() })
}

func TestGroupContract(t *testing.T) {
	repotest.RunGroups(t, func(t *testing.T) domain.GroupRepository { return memory.NewGroupRepository() })
}
