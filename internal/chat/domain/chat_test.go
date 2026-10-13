package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

var t0 = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func TestNewChannel(t *testing.T) {
	c, err := domain.NewChannel("c-1", "w-1", " Dev-Team ", " Sprint talk ", false, "u-lead", t0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "dev-team" || c.Topic != "Sprint talk" || c.Kind != domain.KindChannel || !c.IsMember("u-lead") {
		t.Fatalf("channel = %+v (name lower-cased, creator joins)", c)
	}
	for name, in := range map[string]string{"blank": " ", "spaces": "dev team", "too long": strings.Repeat("a", 81), "symbols": "dev!"} {
		if _, err := domain.NewChannel("c", "w", in, "", false, "u", t0); !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestChannelAccess(t *testing.T) {
	pub, _ := domain.NewChannel("c-1", "w", "general", "", false, "u-lead", t0)
	priv, _ := domain.NewChannel("c-2", "w", "secret", "", true, "u-lead", t0)
	if !pub.CanRead("u-dev") || priv.CanRead("u-dev") {
		t.Fatal("public channels are readable by everyone in the workspace, private only by members")
	}
	if err := pub.Join("u-dev"); err != nil || !pub.IsMember("u-dev") {
		t.Fatalf("join public = %v", err)
	}
	if err := pub.Join("u-dev"); err != nil {
		t.Fatalf("join twice is a no-op, got %v", err)
	}
	if err := priv.Join("u-dev"); !errors.Is(err, domain.ErrNotJoinable) {
		t.Fatalf("join private = %v", err)
	}
	if err := priv.Invite("u-dev", "u-qa"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("outsider invites = %v", err)
	}
	if err := priv.Invite("u-lead", "u-dev"); err != nil || !priv.CanRead("u-dev") {
		t.Fatalf("invite = %v", err)
	}
	if err := priv.Leave("u-dev"); err != nil || priv.IsMember("u-dev") {
		t.Fatalf("leave = %v", err)
	}
	if err := priv.Leave("u-dev"); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("leave twice = %v", err)
	}
}

func TestOpenDM(t *testing.T) {
	dm, err := domain.OpenDM("d-1", "w", "u-b", []domain.UserID{"u-a", "u-b", "u-a"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if dm.Kind != domain.KindDM || !dm.Private || len(dm.Members) != 2 || dm.Key != "u-a,u-b" {
		t.Fatalf("dm = %+v", dm)
	}
	if !dm.CanRead("u-a") || dm.CanRead("u-c") {
		t.Fatal("only DM members read it")
	}
	for name, err := range map[string]error{
		"join":   dm.Join("u-c"),
		"invite": dm.Invite("u-a", "u-c"),
		"leave":  dm.Leave("u-a"),
	} {
		if !errors.Is(err, domain.ErrNotJoinable) {
			t.Errorf("%s: err = %v (DM members are fixed)", name, err)
		}
	}
	if _, err := domain.OpenDM("d", "w", "u-a", []domain.UserID{"u-a"}, t0); !errors.Is(err, domain.ErrDMSize) {
		t.Fatalf("self DM = %v", err)
	}
	many := []domain.UserID{"1", "2", "3", "4", "5", "6", "7", "8"}
	if _, err := domain.OpenDM("d", "w", "u-a", many, t0); !errors.Is(err, domain.ErrDMSize) {
		t.Fatalf("9-person DM = %v", err)
	}
}

func TestMessage(t *testing.T) {
	m, err := domain.NewMessage("m-1", "c-1", "u-a", "  hello  ", t0)
	if err != nil || m.Body != "hello" {
		t.Fatalf("message = %+v %v", m, err)
	}
	for _, body := range []string{" ", strings.Repeat("x", domain.MaxBody+1)} {
		if _, err := domain.NewMessage("m", "c", "u", body, t0); !errors.Is(err, domain.ErrInvalidBody) {
			t.Errorf("body %d chars: err = %v", len(body), err)
		}
	}
	if err := m.Edit("u-b", "hijack", t0); !errors.Is(err, domain.ErrNotAuthor) {
		t.Fatalf("edit by other = %v", err)
	}
	if err := m.Edit("u-a", "hello world", t0.Add(time.Minute)); err != nil || m.Body != "hello world" || !m.EditedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("edit = %+v %v", m, err)
	}
	if err := m.Delete("u-b", t0); !errors.Is(err, domain.ErrNotAuthor) {
		t.Fatalf("delete by other = %v", err)
	}
	if err := m.Delete("u-a", t0.Add(2*time.Minute)); err != nil || !m.Deleted || m.Body != "" {
		t.Fatalf("delete = %+v %v", m, err)
	}
	if err := m.Edit("u-a", "again", t0); !errors.Is(err, domain.ErrDeleted) {
		t.Fatalf("edit deleted = %v", err)
	}
	if err := m.Delete("u-a", t0); !errors.Is(err, domain.ErrDeleted) {
		t.Fatalf("delete twice = %v", err)
	}
}
