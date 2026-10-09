package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

// ADR-0004: a project belongs to the workspace it was created in and is invisible elsewhere.
func TestTenantIsolation(t *testing.T) {
	s := newService(t)
	acme := tenant.With(context.Background(), "w-acme")
	globex := tenant.With(context.Background(), "w-globex")
	p, err := s.Create(acme, alice, "KYB", "Kyber")
	if err != nil || p.Workspace() != "w-acme" {
		t.Fatalf("create = %+v %v", p, err)
	}
	// alice is admin of KYB, yet from another workspace's host the project does not exist.
	if _, err := s.Get(globex, alice, "KYB"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("cross-tenant get err = %v", err)
	}
	if err := s.Authorize(globex, alice, "KYB", domain.PermRead); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("cross-tenant authorize err = %v", err)
	}
	if _, err := s.Members(globex, alice, "KYB"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("cross-tenant members err = %v", err)
	}
	if err := s.SetMember(globex, alice, "KYB", "bob@x.uz", "member"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("cross-tenant set member err = %v", err)
	}
	if list, _ := s.List(globex, alice); len(list) != 0 {
		t.Fatalf("cross-tenant list = %v", list)
	}
	if list, _ := s.List(acme, alice); len(list) != 1 {
		t.Fatalf("own list = %v", list)
	}
	if err := s.Authorize(acme, alice, "KYB", domain.PermAdmin); err != nil {
		t.Fatal(err)
	}
	// Internal callers (relay handlers, jobs) are not tenant-scoped.
	if err := s.Authorize(context.Background(), alice, "KYB", domain.PermRead); err != nil {
		t.Fatal(err)
	}
	// Without a tenant, new projects go to the default workspace (single-tenant mode).
	q, _ := s.Create(context.Background(), bob, "OPS", "Ops")
	if q.Workspace() != domain.DefaultWorkspace {
		t.Fatalf("default workspace = %q", q.Workspace())
	}
}

type joins []string

func (j *joins) Join(_ context.Context, workspace, user string) error {
	*j = append(*j, workspace+"/"+user)
	return nil
}

// Adding someone to a project also lets them into the project's workspace.
func TestSetMemberJoinsWorkspace(t *testing.T) {
	var j joins
	s := newService(t).WithWorkspaces(&j)
	acme := tenant.With(context.Background(), "w-acme")
	_, _ = s.Create(acme, alice, "KYB", "Kyber")
	if err := s.SetMember(acme, alice, "KYB", "bob@x.uz", "member"); err != nil {
		t.Fatal(err)
	}
	if len(j) != 1 || j[0] != "w-acme/u-bob" {
		t.Fatalf("joins = %v", j)
	}
}

func TestWorkspaceOf(t *testing.T) {
	s := newService(t)
	_, _ = s.Create(tenant.With(context.Background(), "w-acme"), alice, "KYB", "Kyber")
	if ws, err := s.WorkspaceOf(context.Background(), "KYB"); err != nil || ws != "w-acme" {
		t.Fatalf("WorkspaceOf = %q %v", ws, err)
	}
	if _, err := s.WorkspaceOf(context.Background(), "NOPE"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}
