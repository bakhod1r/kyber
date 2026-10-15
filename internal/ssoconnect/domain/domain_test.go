package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
)

var now = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

func TestNewConnection(t *testing.T) {
	ok := domain.Settings{Name: "Acme Okta", Issuer: "https://acme.okta.com", ClientID: "cid", ClientSecret: "s3cret"}
	c, err := domain.NewConnection("c-1", ok, now)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "c-1" || c.Kind != domain.KindOIDC || !c.Enabled || c.Name != "Acme Okta" || !c.CreatedAt.Equal(now) {
		t.Fatalf("connection = %+v", c)
	}
	local := ok
	local.Issuer = "http://localhost:8080/realms/dev"
	if _, err := domain.NewConnection("c-2", local, now); err != nil {
		t.Fatalf("localhost over http: %v", err)
	}
	for name, mutate := range map[string]func(*domain.Settings){
		"no name":      func(s *domain.Settings) { s.Name = " " },
		"long name":    func(s *domain.Settings) { s.Name = string(make([]byte, 61)) },
		"no issuer":    func(s *domain.Settings) { s.Issuer = "" },
		"http issuer":  func(s *domain.Settings) { s.Issuer = "http://idp.example.com" },
		"bad issuer":   func(s *domain.Settings) { s.Issuer = "://x" },
		"query issuer": func(s *domain.Settings) { s.Issuer = "https://idp.example.com?x=1" },
		"no client":    func(s *domain.Settings) { s.ClientID = "" },
		"no secret":    func(s *domain.Settings) { s.ClientSecret = "" },
	} {
		s := ok
		mutate(&s)
		if _, err := domain.NewConnection("c-3", s, now); !errors.Is(err, domain.ErrInvalidConnection) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := domain.NewConnection("", ok, now); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("no id: %v", err)
	}
}

func TestRenameAndToggle(t *testing.T) {
	c, _ := domain.NewConnection("c-1", domain.Settings{Name: "A", Issuer: "https://a.example", ClientID: "x", ClientSecret: "y"}, now)
	if err := c.Rename(""); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Fatalf("empty rename: %v", err)
	}
	if err := c.Rename(" Okta "); err != nil || c.Name != "Okta" {
		t.Fatalf("rename: %v %q", err, c.Name)
	}
	c.Disable()
	if c.Enabled {
		t.Fatal("still enabled")
	}
	c.Enable()
	if !c.Enabled {
		t.Fatal("not enabled")
	}
}

func TestProviderName(t *testing.T) {
	if got := domain.ProviderOf("c-1"); got != "sso:c-1" {
		t.Fatalf("provider = %q", got)
	}
}

func TestDomains(t *testing.T) {
	c, _ := domain.NewConnection("c-1", domain.Settings{Name: "A", Issuer: "https://a.example", ClientID: "x", ClientSecret: "y"}, now)
	if err := c.AddDomain(" Acme.UZ ", "tok", false); err != nil {
		t.Fatal(err)
	}
	if len(c.Domains) != 1 || c.Domains[0] != (domain.Domain{Name: "acme.uz", Token: "tok"}) || c.Serves("acme.uz") {
		t.Fatalf("domains = %+v", c.Domains)
	}
	for _, bad := range []string{"", "acme", "-x.uz", "a..uz", "acme.uz", "ACME.uz", "x.uz/path", strings.Repeat("a", 64) + ".uz"} {
		if err := c.AddDomain(bad, "t", false); !errors.Is(err, domain.ErrInvalidConnection) {
			t.Errorf("%q accepted: %v", bad, err)
		}
	}
	if err := c.VerifyDomain("acme.uz", []string{"v=spf1", "kyber-verify=wrong"}); !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Fatalf("wrong token: %v", err)
	}
	if err := c.VerifyDomain("acme.uz", []string{"v=spf1", " kyber-verify=tok "}); err != nil || !c.Serves("ACME.uz") {
		t.Fatalf("verify: %v %+v", err, c.Domains)
	}
	if err := c.VerifyDomain("other.uz", nil); !errors.Is(err, domain.ErrDomainNotFound) {
		t.Fatalf("unknown domain: %v", err)
	}
	if err := c.AddDomain("trusted.uz", "t2", true); err != nil || !c.Serves("trusted.uz") {
		t.Fatalf("trusted: %v", err)
	}
	if err := c.RemoveDomain("acme.uz"); err != nil || c.Serves("acme.uz") || len(c.Domains) != 1 {
		t.Fatalf("remove: %v %+v", err, c.Domains)
	}
	if err := c.RemoveDomain("acme.uz"); !errors.Is(err, domain.ErrDomainNotFound) {
		t.Fatalf("remove twice: %v", err)
	}
}

func TestDomainOf(t *testing.T) {
	for in, want := range map[string]string{"Zara@Acme.UZ": "acme.uz", " a@b.c ": "b.c", "nope": "", "a@": ""} {
		if got := domain.DomainOf(in); got != want {
			t.Errorf("DomainOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnforce(t *testing.T) {
	site, _ := domain.NewConnection("c-1", domain.Settings{Name: "A", Issuer: "https://a.example", ClientID: "x", ClientSecret: "y"}, now)
	if err := site.Enforce(true); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Fatalf("an installation-wide connection cannot be required: %v", err)
	}
	if err := site.Enforce(false); err != nil {
		t.Fatal(err)
	}
	ws, _ := domain.NewConnection("c-2", domain.Settings{Name: "A", Issuer: "https://a.example", ClientID: "x", ClientSecret: "y", Workspace: "w-1"}, now)
	if err := ws.Enforce(true); err != nil || !ws.Enforced || !ws.Requires("w-1") {
		t.Fatalf("enforce = %v %+v", err, ws)
	}
	if ws.Requires("w-2") {
		t.Fatal("required in another workspace")
	}
	ws.Disable()
	if ws.Requires("w-1") {
		t.Fatal("a disabled connection is not required")
	}
}
