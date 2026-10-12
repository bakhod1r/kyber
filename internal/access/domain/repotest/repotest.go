// Package repotest is the contract every permission-scheme Repository adapter must pass.
package repotest

import (
	"context"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

const Other = "00000000-0000-4000-8000-0000000000aa"

// Run needs a store that already has project KYB in the default workspace and in Other.
func Run(t *testing.T, r domain.SchemeRepository) {
	ctx := tenant.With(context.Background(), tenant.Default)
	if _, ok, err := r.ProjectGrants(ctx, "KYB"); ok || err != nil {
		t.Fatalf("fresh project has a scheme: %v %v", ok, err)
	}
	table := domain.DefaultScheme().Table()
	table[domain.CreateIssues] = []domain.Grant{domain.Role(domain.RoleAdmin), domain.User("u-1"), domain.Reporter()}
	table[domain.CloseIssues] = []domain.Grant{domain.Assignee()}
	table[domain.ManageSprints] = nil
	if err := r.SaveProjectGrants(ctx, "KYB", table); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.ProjectGrants(ctx, "KYB")
	if !ok || err != nil {
		t.Fatalf("saved scheme: %v %v", ok, err)
	}
	for p, want := range table {
		if !sameGrants(got[p], want) {
			t.Errorf("%s = %v, want %v", p, got[p], want)
		}
	}
	// Saving again replaces, not merges.
	table[domain.CreateIssues] = []domain.Grant{domain.Role(domain.RoleAdmin)}
	if err := r.SaveProjectGrants(ctx, "KYB", table); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.ProjectGrants(ctx, "KYB"); !sameGrants(got[domain.CreateIssues], table[domain.CreateIssues]) {
		t.Fatalf("replaced = %v", got[domain.CreateIssues])
	}
	// Workspaces are isolated.
	other := tenant.With(context.Background(), Other)
	if _, ok, _ := r.ProjectGrants(other, "KYB"); ok {
		t.Fatal("scheme leaked into another workspace")
	}
	if err := r.DeleteProjectGrants(other, "KYB"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.ProjectGrants(ctx, "KYB"); !ok {
		t.Fatal("deleting in another workspace removed this one")
	}
	if err := r.DeleteProjectGrants(ctx, "KYB"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.ProjectGrants(ctx, "KYB"); ok {
		t.Fatal("reset project still has a scheme")
	}
}

func sameGrants(a, b []domain.Grant) bool {
	key := func(g domain.Grant) string { return g.Kind() + ":" + g.Value() }
	ka, kb := make([]string, len(a)), make([]string, len(b))
	for i, g := range a {
		ka[i] = key(g)
	}
	for i, g := range b {
		kb[i] = key(g)
	}
	slices.Sort(ka)
	slices.Sort(kb)
	return slices.Equal(ka, kb)
}
