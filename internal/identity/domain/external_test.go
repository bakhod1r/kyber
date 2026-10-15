package domain_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func TestProviderValid(t *testing.T) {
	for p, want := range map[domain.Provider]bool{
		domain.ProviderGoogle: true, domain.ProviderSAML: true, "sso:75000000-0000-4000-8000-000000000001": true,
		"sso:": false, "facebook": false, "": false,
	} {
		if got := p.Valid(); got != want {
			t.Errorf("%q.Valid() = %v, want %v", p, got, want)
		}
	}
}
