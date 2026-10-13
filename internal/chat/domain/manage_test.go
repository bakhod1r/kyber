package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestChannelManagement(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	c, _ := domain.NewChannel("c-1", "ws", "general", "", false, "owner", t0)
	_ = c.Join("admin")
	_ = c.Join("dev")
	if err := c.MakeAdmin("dev", "admin"); !errors.Is(err, domain.ErrNotOwner) {
		t.Fatalf("non-owner makes admin = %v", err)
	}
	if err := c.MakeAdmin("owner", "outsider"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("admin who is not a member = %v", err)
	}
	if err := c.MakeAdmin("owner", "admin"); err != nil || !c.CanManage("admin") || c.CanManage("dev") || !c.CanManage("owner") {
		t.Fatalf("make admin = %v %+v", err, c.Admins)
	}
	if err := c.Rename("dev", "team", "x"); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("member renames = %v", err)
	}
	if err := c.Rename("admin", "Team-Core", "Core team"); err != nil || c.Name != "team-core" || c.Topic != "Core team" {
		t.Fatalf("rename = %v %+v", err, c)
	}
	if err := c.Rename("admin", "bad name!", ""); !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("bad name = %v", err)
	}
	if err := c.Remove("admin", "owner"); !errors.Is(err, domain.ErrNotOwner) {
		t.Fatalf("removing the owner = %v", err)
	}
	if err := c.Remove("admin", "dev"); err != nil || c.IsMember("dev") {
		t.Fatalf("remove = %v", err)
	}
	if err := c.Archive("dev", t0); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("outsider archives = %v", err)
	}
	if err := c.Archive("admin", t0); err != nil || !c.Archived() {
		t.Fatalf("archive = %v", err)
	}
	if err := c.Writable(); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("archived writable = %v", err)
	}
	if err := c.Join("dev"); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("join archived = %v", err)
	}
	if err := c.Unarchive("owner"); err != nil || c.Archived() || c.Writable() != nil {
		t.Fatalf("unarchive = %v", err)
	}
	if err := c.RemoveAdmin("owner", "admin"); err != nil || c.CanManage("admin") {
		t.Fatalf("remove admin = %v", err)
	}
	dm, _ := domain.OpenDM("d-1", "ws", "a", []domain.UserID{"b"}, t0)
	if err := dm.Rename("a", "x", ""); !errors.Is(err, domain.ErrNotJoinable) {
		t.Fatalf("rename a DM = %v", err)
	}
	if err := c.LinkProject("dev", "KYB"); !errors.Is(err, domain.ErrNotManager) {
		t.Fatalf("member links = %v", err)
	}
	if err := c.LinkProject("owner", "kyb"); err != nil || c.Project != "KYB" {
		t.Fatalf("link = %v %q", err, c.Project)
	}
	if err := c.LinkProject("owner", "not a key"); !errors.Is(err, domain.ErrInvalidProject) {
		t.Fatalf("bad key = %v", err)
	}
	if err := c.LinkProject("owner", ""); err != nil || c.Project != "" {
		t.Fatalf("unlink = %v", err)
	}
}
