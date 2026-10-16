package memory_test

import (
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/project/domain/repotest"
	"github.com/bakhod1r/kyber/internal/project/infrastructure/memory"
)

func TestInvitationContract(t *testing.T) {
	repotest.RunInvitations(t, func(*testing.T) domain.InvitationRepository { return memory.NewInvitations() },
		func(n int) domain.InvitationID { return domain.InvitationID(fmt.Sprintf("i-%d", n)) })
}
