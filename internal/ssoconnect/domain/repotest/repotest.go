// Package repotest is the contract every SSO connection repository must pass.
package repotest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

// IDs are UUIDs so the PostgreSQL adapter can store them.
const (
	C1, C2 = "75000000-0000-4000-8000-000000000001", "75000000-0000-4000-8000-000000000002"
	// WS is a workspace the adapter's test database must hold (workspace connections reference it).
	WS = "76000000-0000-4000-8000-000000000001"
)

func Run(t *testing.T, repo domain.Connections) {
	ctx := context.Background()
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	c1, _ := domain.NewConnection(C1, domain.Settings{Name: "Okta", Issuer: "https://acme.okta.com", ClientID: "a", ClientSecret: "s1", GroupsClaim: "roles"}, at)
	c2, _ := domain.NewConnection(C2, domain.Settings{Name: "Entra", Issuer: "https://login.microsoftonline.com/x/v2.0", ClientID: "b", ClientSecret: "s2", Workspace: WS}, at.Add(time.Minute))
	_ = c2.Enforce(true)
	for _, c := range []*domain.Connection{c2, c1} {
		if err := repo.Save(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	all, err := repo.All(ctx)
	if err != nil || len(all) != 2 || all[0].ID != C1 || all[1].ID != C2 || !reflect.DeepEqual(all[1], *c2) {
		t.Fatalf("all (oldest first) = %+v %v", all, err)
	}
	if !reflect.DeepEqual(all[0], *c1) {
		t.Fatalf("round trip = %+v, want %+v", all[0], *c1)
	}
	_ = c1.AddDomain("acme.uz", "tok1", false)
	_ = c1.AddDomain("acme.com", "tok2", true)
	if err := repo.Save(ctx, c1); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.ByDomain(ctx, "ACME.uz"); err != nil || got.ID != C1 || !reflect.DeepEqual(got.Domains, c1.Domains) {
		t.Fatalf("by domain = %+v %v", got, err)
	}
	if _, err := repo.ByDomain(ctx, "nobody.uz"); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Fatalf("unknown domain: %v", err)
	}
	_ = c1.VerifyDomain("acme.uz", []string{"kyber-verify=tok1"})
	_ = c1.RemoveDomain("acme.com")
	if err := repo.Save(ctx, c1); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.ByID(ctx, C1); len(got.Domains) != 1 || !got.Domains[0].Verified {
		t.Fatalf("domains after save = %+v", got.Domains)
	}
	if _, err := repo.ByDomain(ctx, "acme.com"); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Fatalf("removed domain still routes: %v", err)
	}
	if req, err := repo.RequiredBy(ctx, WS); err != nil || len(req) != 1 || req[0].ID != C2 {
		t.Fatalf("required by %s = %+v %v", WS, req, err)
	}
	if req, err := repo.RequiredBy(ctx, "77000000-0000-4000-8000-000000000001"); err != nil || len(req) != 0 {
		t.Fatalf("required by another workspace = %+v %v", req, err)
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
	_ = c2.AddDomain("acme.uz", "tok3", false)
	if err := repo.Save(ctx, c2); !errors.Is(err, domain.ErrDomainTaken) {
		t.Fatalf("a domain has one connection: %v", err)
	}
	if got, _ := repo.ByID(ctx, C2); len(got.Domains) != 0 {
		t.Fatalf("a refused save was kept: %+v", got.Domains)
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
