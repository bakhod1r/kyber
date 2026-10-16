package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

// B10: an archived project is read-only: members still browse it, nobody changes it until an admin restores it.
func TestArchivedProjectIsReadOnly(t *testing.T) {
	p, _ := domain.NewProject("p-1", domain.DefaultWorkspace, "KYB", "Kyber", "u-admin")
	_ = p.SetMember("u-dev", domain.RoleMember)
	if err := p.Archive("u-dev"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member archives: %v", err)
	}
	if err := p.Archive("u-admin"); err != nil {
		t.Fatal(err)
	}
	if !p.Archived() {
		t.Fatal("not archived")
	}
	if err := p.Authorize("u-dev", domain.PermRead); err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, need := range []domain.Permission{domain.PermWrite, domain.PermAdmin} {
		if err := p.Authorize("u-admin", need); !errors.Is(err, domain.ErrArchived) {
			t.Fatalf("need %d: %v", need, err)
		}
	}
	if err := p.Authorize("u-stranger", domain.PermRead); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("stranger: %v", err)
	}
	if err := p.SetMember("u-qa", domain.RoleViewer); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("membership change: %v", err)
	}
	if err := p.Archive("u-admin"); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("archive twice: %v", err)
	}
	if err := p.Restore("u-dev"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member restores: %v", err)
	}
	if err := p.Restore("u-admin"); err != nil || p.Archived() {
		t.Fatalf("restore: %v", err)
	}
	if err := p.Authorize("u-dev", domain.PermWrite); err != nil {
		t.Fatalf("write after restore: %v", err)
	}
}
