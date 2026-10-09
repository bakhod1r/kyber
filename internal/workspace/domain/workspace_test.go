package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/workspace/domain"
)

func TestParseSlug(t *testing.T) {
	for in, want := range map[string]string{"acme": "acme", " Acme-Corp ": "acme-corp", "a1b": "a1b", "x-y-z-2026": "x-y-z-2026"} {
		if got, err := domain.ParseSlug(in); err != nil || got != want {
			t.Errorf("ParseSlug(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "ab", "-acme", "acme-", "ac_me", "acme.corp", "ünicode", "a--b", "www", "api", "admin", "default", "x123456789012345678901234567890123"} {
		if _, err := domain.ParseSlug(bad); !errors.Is(err, domain.ErrInvalidSlug) {
			t.Errorf("ParseSlug(%q) err = %v", bad, err)
		}
	}
}

func TestNewWorkspace(t *testing.T) {
	w, err := domain.NewWorkspace("w-1", "acme", " Acme Corp ", "u-1")
	if err != nil || w.ID() != "w-1" || w.Slug() != "acme" || w.Name() != "Acme Corp" {
		t.Fatalf("w = %+v, %v", w, err)
	}
	if r, ok := w.RoleOf("u-1"); !ok || r != domain.RoleOwner {
		t.Fatalf("creator role = %s %v", r, ok)
	}
	if _, err := domain.NewWorkspace("w-2", "acme", " ", "u-1"); !errors.Is(err, domain.ErrEmptyName) {
		t.Fatalf("empty name err = %v", err)
	}
	if _, err := domain.NewWorkspace("w-2", "www", "W", "u-1"); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("reserved err = %v", err)
	}
}

func TestMembers(t *testing.T) {
	w, _ := domain.NewWorkspace("w-1", "acme", "Acme", "u-1")
	if err := w.AddMember("u-2", domain.RoleMember); err != nil {
		t.Fatal(err)
	}
	if err := w.AddMember("u-2", domain.RoleAdmin); err != nil {
		t.Fatal(err) // changing a role is allowed
	}
	if r, _ := w.RoleOf("u-2"); r != domain.RoleAdmin {
		t.Fatalf("role = %s", r)
	}
	if err := w.AddMember("u-3", "god"); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("bad role err = %v", err)
	}
	if err := w.AddMember("u-1", domain.RoleMember); !errors.Is(err, domain.ErrLastOwner) {
		t.Fatalf("demoting the last owner err = %v", err)
	}
	if !w.IsMember("u-2") || w.IsMember("u-9") || len(w.Members()) != 2 {
		t.Fatalf("members = %v", w.Members())
	}
	got := domain.Rehydrate("w-1", "acme", "Acme", w.Members())
	if !got.IsMember("u-2") || got.Slug() != "acme" {
		t.Fatalf("rehydrate = %+v", got)
	}
	if r, err := domain.ParseRole("owner"); err != nil || r != domain.RoleOwner {
		t.Fatalf("ParseRole = %s %v", r, err)
	}
}

func TestDefaultWorkspace(t *testing.T) {
	if domain.DefaultID == "" || domain.DefaultSlug != "default" {
		t.Fatal("single-tenant installs live in the default workspace")
	}
}
