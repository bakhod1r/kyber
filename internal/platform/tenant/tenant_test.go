package tenant_test

import (
	"context"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func TestTenant(t *testing.T) {
	if _, ok := tenant.From(context.Background()); ok {
		t.Fatal("unscoped")
	}
	if id, ok := tenant.From(tenant.With(context.Background(), "w-1")); !ok || id != "w-1" {
		t.Fatalf("scoped = %q %v", id, ok)
	}
}

func TestWorkspaceDefault(t *testing.T) {
	if tenant.Workspace(context.Background()) != tenant.Default || tenant.Workspace(tenant.With(context.Background(), "w")) != "w" {
		t.Fatal("Workspace")
	}
}
