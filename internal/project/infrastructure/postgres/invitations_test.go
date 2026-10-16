package postgres_test

import (
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/project/domain"
	"github.com/bakhod1r/kyber/internal/project/domain/repotest"
	"github.com/bakhod1r/kyber/internal/project/infrastructure/postgres"
)

func TestInvitationContract(t *testing.T) {
	repotest.RunInvitations(t, func(t *testing.T) domain.InvitationRepository { return postgres.NewInvitations(dbtest.New(t)) },
		func(n int) domain.InvitationID {
			return domain.InvitationID(fmt.Sprintf("40000000-0000-4000-8000-%012d", n))
		})
}
