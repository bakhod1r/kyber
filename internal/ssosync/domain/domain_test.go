package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssosync/domain"
)

var at = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

func ws(w string) domain.Target { return domain.Target{Scope: domain.ScopeWorkspace, Workspace: w} }
func proj(w, k string) domain.Target {
	return domain.Target{Scope: domain.ScopeProject, Workspace: w, Project: k}
}

func mapping(t *testing.T, group string, target domain.Target, role string) domain.Mapping {
	t.Helper()
	m, err := domain.NewMapping(domain.MappingID("m-"+group+role), group, target, role, "u-admin", at)
	if err != nil {
		t.Fatal(err)
	}
	return *m
}

func TestNewMapping(t *testing.T) {
	if m, err := domain.NewMapping("m-1", " Engineers ", proj("w-1", "kyb"), "member", "u-a", at); err != nil || m.Group != "Engineers" || m.Target.Project != "KYB" {
		t.Fatalf("mapping = %+v %v", m, err)
	}
	bad := []struct {
		group  string
		target domain.Target
		role   string
	}{
		{"", ws("w-1"), "member"},
		{"g", domain.Target{Scope: "galaxy", Workspace: "w-1"}, "member"},
		{"g", domain.Target{Scope: domain.ScopeWorkspace}, "member"},
		{"g", ws("w-1"), "owner"},  // owners stay a human decision
		{"g", ws("w-1"), "viewer"}, // workspaces have no viewers
		{"g", domain.Target{Scope: domain.ScopeProject, Workspace: "w-1"}, "member"},
		{"g", proj("w-1", "KYB"), "boss"},
	}
	for _, b := range bad {
		if _, err := domain.NewMapping("m", b.group, b.target, b.role, "u", at); !errors.Is(err, domain.ErrInvalidMapping) {
			t.Errorf("%+v: %v", b, err)
		}
	}
}

func TestPlan(t *testing.T) {
	maps := []domain.Mapping{
		mapping(t, "eng", ws("w-1"), "member"),
		mapping(t, "eng-leads", ws("w-1"), "admin"),
		mapping(t, "eng", proj("w-1", "KYB"), "member"),
		mapping(t, "qa", proj("w-1", "KYB"), "viewer"),
		mapping(t, "ops", proj("w-1", "OPS"), "admin"),
	}
	held := []domain.Grant{
		{User: "u-1", Target: proj("w-1", "KYB"), Role: "member"}, // unchanged
		{User: "u-1", Target: proj("w-1", "OPS"), Role: "admin"},  // no longer in ops: revoke
		{User: "u-1", Target: ws("w-1"), Role: "member"},          // now a lead: upgrade
	}
	set, revoke := domain.Plan("u-1", maps, []string{"ENG", "eng-leads", "qa", "unmapped"}, held)
	if len(set) != 1 || set[0].Target != ws("w-1") || set[0].Role != "admin" {
		t.Fatalf("set = %+v (highest role wins, groups match case-insensitively)", set)
	}
	if len(revoke) != 1 || revoke[0].Target != proj("w-1", "OPS") {
		t.Fatalf("revoke = %+v", revoke)
	}
	set, revoke = domain.Plan("u-2", maps, nil, nil)
	if len(set)+len(revoke) != 0 {
		t.Fatalf("no groups, nothing held: %v %v", set, revoke)
	}
}
