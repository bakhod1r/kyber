package memory_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/domain/externaltest"
)

func TestExternalIdentitiesContract(t *testing.T) {
	externaltest.Run(t, func(*testing.T) (domain.ExternalIdentities, func(domain.UserID)) {
		r := memory.NewRepository()
		return r, func(id domain.UserID) {
			u, _ := domain.NewUser(id, domain.Email(string(id)+"@x.uz"), "U", "")
			_ = r.Create(context.Background(), u)
		}
	})
}
