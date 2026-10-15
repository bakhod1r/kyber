// Package repotest is the contract every ssosync repository must pass.
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

// IDs are UUIDs so the PostgreSQL adapter can store them.
const (
	M1, M2 = "72000000-0000-4000-8000-000000000001", "72000000-0000-4000-8000-000000000002"
	WS     = "73000000-0000-4000-8000-000000000001"
	User   = "74000000-0000-4000-8000-000000000001"
	Admin  = "74000000-0000-4000-8000-000000000002"
)

func Run(t *testing.T, maps domain.Mappings, grants domain.Grants) {
	ctx := context.Background()
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	wsT := domain.Target{Scope: domain.ScopeWorkspace, Workspace: WS}
	pT := domain.Target{Scope: domain.ScopeProject, Workspace: WS, Project: "KYB"}
	m1, _ := domain.NewMapping(M1, "eng", wsT, "member", Admin, at)
	m2, _ := domain.NewMapping(M2, "eng", pT, "admin", Admin, at.Add(time.Minute))
	for _, m := range []*domain.Mapping{m2, m1} {
		if err := maps.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	all, err := maps.All(ctx)
	if err != nil || len(all) != 2 || all[0].ID != M1 || all[1].Target != pT || all[1].Role != "admin" || !all[0].CreatedAt.Equal(at) || all[0].CreatedBy != Admin {
		t.Fatalf("all (oldest first) = %+v %v", all, err)
	}
	if err := maps.Delete(ctx, M1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []domain.MappingID{M1, "not-a-uuid"} {
		if err := maps.Delete(ctx, id); !errors.Is(err, domain.ErrMappingNotFound) {
			t.Fatalf("delete missing %s: %v", id, err)
		}
	}

	if err := grants.Save(ctx, domain.Grant{User: User, Target: wsT, Role: "member"}); err != nil {
		t.Fatal(err)
	}
	if err := grants.Save(ctx, domain.Grant{User: User, Target: pT, Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	if err := grants.Save(ctx, domain.Grant{User: User, Target: pT, Role: "admin"}); err != nil { // replaces
		t.Fatal(err)
	}
	held, err := grants.Of(ctx, User)
	if err != nil || len(held) != 2 {
		t.Fatalf("held = %+v %v", held, err)
	}
	for _, g := range held {
		if g.Target == pT && g.Role != "admin" {
			t.Fatalf("not replaced: %+v", g)
		}
	}
	if err := grants.Remove(ctx, User, pT); err != nil {
		t.Fatal(err)
	}
	if held, _ := grants.Of(ctx, User); len(held) != 1 || held[0].Target != wsT {
		t.Fatalf("after delete = %+v", held)
	}
	if held, _ := grants.Of(ctx, Admin); len(held) != 0 {
		t.Fatalf("other user = %+v", held)
	}
}
