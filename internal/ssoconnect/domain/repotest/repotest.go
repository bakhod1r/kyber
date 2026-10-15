// Package repotest is the contract every SSO connection repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// IDs are UUIDs so the PostgreSQL adapter can store them.
const C1, C2 = "75000000-0000-4000-8000-000000000001", "75000000-0000-4000-8000-000000000002"

func Run(t *testing.T, repo domain.Connections) {
	ctx := context.Background()
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	c1, _ := domain.NewConnection(C1, domain.Settings{Name: "Okta", Issuer: "https://acme.okta.com", ClientID: "a", ClientSecret: "s1", GroupsClaim: "roles"}, at)
	c2, _ := domain.NewConnection(C2, domain.Settings{Name: "Entra", Issuer: "https://login.microsoftonline.com/x/v2.0", ClientID: "b", ClientSecret: "s2"}, at.Add(time.Minute))
	for _, c := range []*domain.Connection{c2, c1} {
		if err := repo.Save(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	all, err := repo.All(ctx)
	if err != nil || len(all) != 2 || all[0].ID != C1 || all[1].ID != C2 {
		t.Fatalf("all (oldest first) = %+v %v", all, err)
	}
	if all[0] != *c1 {
		t.Fatalf("round trip = %+v, want %+v", all[0], *c1)
	}
	c1.Disable()
	_ = c1.Rename("Okta (old)")
	if err := repo.Save(ctx, c1); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ByID(ctx, C1)
	if err != nil || got.Enabled || got.Name != "Okta (old)" || got.ClientSecret != "s1" {
		t.Fatalf("updated = %+v %v", got, err)
	}
	if err := repo.Delete(ctx, C1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.ConnectionID{C1, "not-a-uuid"} {
		if _, err := repo.ByID(ctx, id); !errors.Is(err, domain.ErrConnectionNotFound) {
			t.Errorf("ByID(%s): %v", id, err)
		}
		if err := repo.Delete(ctx, id); !errors.Is(err, domain.ErrConnectionNotFound) {
			t.Errorf("Delete(%s): %v", id, err)
		}
	}
}
