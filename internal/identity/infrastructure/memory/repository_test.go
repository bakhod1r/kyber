package memory_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/domain/externaltest"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
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

func TestOTPContract(t *testing.T) { externaltest.RunOTP(t, memory.NewRepository()) }

func TestConnectContract(t *testing.T) {
	externaltest.RunConnect(t, memory.NewRepository(), "u-1")
}

func TestTokensContract(t *testing.T) {
	externaltest.RunTokens(t, memory.NewRepository(), "u-1", "u-2")
}

func TestOAuthContract(t *testing.T) { externaltest.RunOAuth(t, memory.NewRepository(), "u-1") }
